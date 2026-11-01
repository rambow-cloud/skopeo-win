//go:build ignore

// verify-windows-binary verifies that a Skopeo PE executable was built with
// the pure-Go configuration used by this repository.
package main

import (
	"debug/buildinfo"
	"debug/pe"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
)

var forbiddenDLLFragments = []string{
	"gpgme",
	"gpg-error",
	"assuan",
	"libcrypto",
	"libssl",
	"libgcc",
	"libstdc++",
	"msvcr",
	"ucrtbase",
	"vcruntime",
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "verification failed: "+format+"\n", args...)
	os.Exit(1)
}

func main() {
	if len(os.Args) != 3 {
		fail("usage: go run ./hack/verify-windows-binary.go BINARY ARCH")
	}
	path, expectedArch := os.Args[1], os.Args[2]

	info, err := buildinfo.ReadFile(path)
	if err != nil {
		fail("read Go build metadata: %v", err)
	}
	settings := map[string]string{}
	for _, setting := range info.Settings {
		settings[setting.Key] = setting.Value
	}
	for key, expected := range map[string]string{
		"CGO_ENABLED": "0",
		"GOOS":        "windows",
		"GOARCH":      expectedArch,
	} {
		if actual := settings[key]; actual != expected {
			fail("%s=%q, expected %q", key, actual, expected)
		}
	}
	tags := strings.Split(settings["-tags"], ",")
	if !slices.Contains(tags, "containers_image_openpgp") {
		fail("pure-Go OpenPGP build tag is missing")
	}

	file, err := pe.Open(path)
	if err != nil {
		fail("open PE executable: %v", err)
	}
	defer file.Close()
	expectedMachine := map[string]uint16{
		"amd64": pe.IMAGE_FILE_MACHINE_AMD64,
		"arm64": pe.IMAGE_FILE_MACHINE_ARM64,
	}[expectedArch]
	if expectedMachine == 0 {
		fail("unsupported expected architecture %q", expectedArch)
	}
	if file.FileHeader.Machine != expectedMachine {
		fail("PE machine %#x, expected %#x for %s", file.FileHeader.Machine, expectedMachine, expectedArch)
	}

	// debug/pe.ImportedLibraries is a stub. ImportedSymbols contains the actual
	// loader import entries as function:DLL strings.
	imports, err := file.ImportedSymbols()
	if err != nil {
		fail("read PE imports: %v", err)
	}
	seenLibraries := map[string]bool{}
	for _, imported := range imports {
		separator := strings.LastIndex(imported, ":")
		if separator < 0 {
			fail("unexpected PE import entry %q", imported)
		}
		seenLibraries[strings.ToLower(imported[separator+1:])] = true
	}
	libraries := make([]string, 0, len(seenLibraries))
	for library := range seenLibraries {
		libraries = append(libraries, library)
	}
	sort.Strings(libraries)
	for _, library := range libraries {
		lower := strings.ToLower(library)
		for _, fragment := range forbiddenDLLFragments {
			if strings.Contains(lower, fragment) {
				fail("PE imports forbidden C library %q", library)
			}
		}
	}

	fmt.Printf("verified %s: windows/%s, CGO disabled, pure-Go OpenPGP, PE imports: %s\n",
		path, expectedArch, strings.Join(libraries, ", "))
}
