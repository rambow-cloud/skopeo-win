// Package notices inventories the sources of notices shipped with Store binaries.
package notices

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"slices"
	"strings"
)

const Directory = "ThirdPartyNotices"

type Module struct {
	Path    string `json:"path"`
	Version string `json:"version"`
}

type Binary struct {
	File      string   `json:"file"`
	SHA256    string   `json:"sha256"`
	GoVersion string   `json:"goVersion"`
	GOARCH    string   `json:"goarch"`
	Modules   []Module `json:"modules"`
}

type Notice struct {
	File   string `json:"file"`
	Source string `json:"source"`
	SHA256 string `json:"sha256"`
}

type Component struct {
	Module
	SourceURL string   `json:"sourceUrl"`
	Notices   []Notice `json:"notices"`
}

type Inventory struct {
	SchemaVersion        int         `json:"schemaVersion"`
	VendorManifestSHA256 string      `json:"vendorManifestSHA256"`
	Binaries             []Binary    `json:"binaries"`
	Components           []Component `json:"components"`
}

type input struct {
	binary Binary
	info   *debug.BuildInfo
}

var noticeName = regexp.MustCompile(`(?i)(^|[_-])(licen[sc]es?|notices?|copying|copyrights?|patents|authors|contributors|unlicense)([._-].*)?$`)
var licenseName = regexp.MustCompile(`(?i)^(licen[sc]es?|copying|unlicense)([._-].*)?$`)

// leadingNotices returns full comment blocks, preserving their original bytes.
// It also handles the C-style headers used by Go assembly and header files.
func leadingNotices(data []byte) [][]byte {
	var found [][]byte
	for offset := 0; offset < len(data); {
		start := offset
		for start < len(data) && strings.ContainsRune(" \t\r\n", rune(data[start])) {
			start++
		}
		end := start
		switch {
		case bytes.HasPrefix(data[start:], []byte("/*")):
			last := bytes.Index(data[start+2:], []byte("*/"))
			if last < 0 {
				return found
			}
			end = start + 2 + last + 2
		case bytes.HasPrefix(data[start:], []byte("//")):
			for end < len(data) {
				last := bytes.IndexByte(data[end:], '\n')
				if last < 0 {
					end = len(data)
					break
				}
				end += last + 1
				if !bytes.HasPrefix(bytes.TrimLeft(data[end:], " \t"), []byte("//")) {
					break
				}
			}
		default:
			return found
		}
		text := bytes.ToLower(data[start:end])
		if bytes.Contains(text, []byte("copyright")) || bytes.Contains(text, []byte("spdx-license-identifier")) {
			found = append(found, data[start:end])
		}
		offset = end
	}
	return found
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func readInputs(layout string) ([]input, error) {
	var inputs []input
	for _, name := range []string{"skopeo.exe", "store-launcher.exe"} {
		file := filepath.Join(layout, name)
		info, err := buildinfo.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read build information for %s: %w", name, err)
		}
		command := "skopeo"
		if name == "store-launcher.exe" {
			command = "store-launcher"
		}
		if info.Main.Path != "go.podman.io/skopeo" || info.Path != "go.podman.io/skopeo/cmd/"+command || info.Main.Replace != nil {
			return nil, fmt.Errorf("unexpected main module or command in %s", name)
		}
		settings := map[string]string{}
		for _, setting := range info.Settings {
			settings[setting.Key] = setting.Value
		}
		if settings["GOOS"] != "windows" || (settings["GOARCH"] != "amd64" && settings["GOARCH"] != "arm64") || settings["CGO_ENABLED"] != "0" {
			return nil, fmt.Errorf("%s is not a supported pure-Go Windows binary", name)
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		inputs = append(inputs, input{Binary{File: name, SHA256: digest(data), GoVersion: info.GoVersion, GOARCH: settings["GOARCH"]}, info})
	}
	if inputs[0].binary.GOARCH != inputs[1].binary.GOARCH {
		return nil, fmt.Errorf("CLI and launcher architectures differ")
	}
	return inputs, nil
}

func vendorVersions(data []byte) (map[string]string, error) {
	versions := map[string]string{}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 3 && fields[0] == "#" {
			if slices.Contains(fields, "=>") {
				return nil, fmt.Errorf("replacement modules require a separate notice-source review")
			}
			if _, exists := versions[fields[1]]; exists {
				return nil, fmt.Errorf("duplicate vendor module %q", fields[1])
			}
			versions[fields[1]] = fields[2]
		}
	}
	return versions, scanner.Err()
}

// collect preserves notice bytes and leading Go copyright/license comments.
// Nested modules are inventoried independently, never attributed to their parent.
func collect(root, prefix, origin string, requireLicense bool, skip func(string) bool) (map[string][]byte, []Notice, error) {
	files := map[string][]byte{}
	var notices []Notice
	var copyrights bytes.Buffer
	hasLicense := false
	err := filepath.WalkDir(root, func(file string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, file)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if rel != "." && (entry.Name() == "testdata" || skip(rel)) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		isNotice := noticeName.MatchString(entry.Name())
		isSource := slices.Contains([]string{".go", ".s", ".h", ".c", ".cc", ".cpp"}, strings.ToLower(filepath.Ext(entry.Name())))
		if !isNotice && !isSource {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("notice source is not a regular file: %s/%s", origin, rel)
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if isNotice {
			name := path.Join(prefix, rel)
			files[name] = data
			notices = append(notices, Notice{name, path.Join(origin, rel), digest(data)})
			if !strings.Contains(rel, "/") && licenseName.MatchString(entry.Name()) {
				hasLicense = true
			}
		}
		if isSource {
			for _, block := range leadingNotices(data) {
				fmt.Fprintf(&copyrights, "\nSource: %s\n", path.Join(origin, rel))
				copyrights.Write(block)
				copyrights.WriteByte('\n')
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	if requireLicense && !hasLicense {
		return nil, nil, fmt.Errorf("missing root license in %s; review the matching dependency sources", origin)
	}
	if copyrights.Len() != 0 {
		name := path.Join(prefix, "SOURCE-COPYRIGHTS.txt")
		if _, exists := files[name]; exists {
			return nil, nil, fmt.Errorf("generated copyright filename collides with a source notice: %s", name)
		}
		files[name] = copyrights.Bytes()
		notices = append(notices, Notice{name, origin + " (leading source notices)", digest(copyrights.Bytes())})
	}
	slices.SortFunc(notices, func(a, b Notice) int { return strings.Compare(a.File, b.File) })
	return files, notices, nil
}

// Some hosted Go installations omit root legal files. Only an explicitly
// reviewed snapshot for that exact Go release can supply the missing bytes.
func goRootNotices(repository, version string, files map[string][]byte, notices []Notice) ([]Notice, error) {
	for _, name := range []string{"LICENSE", "PATENTS"} {
		file := "go/" + name
		if _, exists := files[file]; exists {
			continue
		}
		origin := "packaging/store/go-notices/" + version
		data, err := os.ReadFile(filepath.Join(repository, filepath.FromSlash(origin), "source.json"))
		if err != nil {
			return nil, fmt.Errorf("missing Go %s and no reviewed %s snapshot: %w", name, version, err)
		}
		var source struct {
			GoVersion  string            `json:"goVersion"`
			Repository string            `json:"repository"`
			Commit     string            `json:"commit"`
			SHA256     map[string]string `json:"sha256"`
		}
		if err := json.Unmarshal(data, &source); err != nil {
			return nil, err
		}
		if source.GoVersion != version || source.Repository != "https://github.com/golang/go" || !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(source.Commit) {
			return nil, fmt.Errorf("invalid pinned Go notice provenance for %s", version)
		}
		data, err = os.ReadFile(filepath.Join(repository, filepath.FromSlash(origin), name))
		if err != nil {
			return nil, err
		}
		if source.SHA256[name] != digest(data) {
			return nil, fmt.Errorf("pinned Go %s checksum mismatch", name)
		}
		files[file] = data
		notices = append(notices, Notice{file, origin + "/" + name + " (" + source.Repository + "/blob/" + source.Commit + "/" + name + ")", digest(data)})
	}
	slices.SortFunc(notices, func(a, b Notice) int { return strings.Compare(a.File, b.File) })
	return notices, nil
}

func buildFiles(repository, goroot string, inputs []input) (map[string][]byte, error) {
	vendorData, err := os.ReadFile(filepath.Join(repository, "vendor", "modules.txt"))
	if err != nil {
		return nil, err
	}
	versions, err := vendorVersions(vendorData)
	if err != nil {
		return nil, err
	}
	toolchainData, err := os.ReadFile(filepath.Join(goroot, "VERSION"))
	if err != nil {
		return nil, err
	}
	toolchainFields := strings.Fields(string(toolchainData))
	if len(toolchainFields) == 0 {
		return nil, fmt.Errorf("Go source VERSION is empty")
	}
	goVersion := toolchainFields[0]
	inventory := Inventory{SchemaVersion: 1, VendorManifestSHA256: digest(vendorData)}
	modules := map[string]string{}
	for _, in := range inputs {
		if in.info.GoVersion != goVersion {
			return nil, fmt.Errorf("%s uses %s, notice sources use %s; historical packaging requires matching Go and dependency sources", in.binary.File, in.info.GoVersion, goVersion)
		}
		binary := in.binary
		for _, module := range in.info.Deps {
			if module.Replace != nil || module.Version == "" || versions[module.Path] != module.Version || !fs.ValidPath(module.Path) || strings.ContainsAny(module.Path, `:\`) {
				return nil, fmt.Errorf("cannot match dependency %s@%s in %s to vendor/modules.txt; use a matching source checkout", module.Path, module.Version, binary.File)
			}
			modules[module.Path] = module.Version
			binary.Modules = append(binary.Modules, Module{module.Path, module.Version})
		}
		slices.SortFunc(binary.Modules, func(a, b Module) int { return strings.Compare(a.Path, b.Path) })
		inventory.Binaries = append(inventory.Binaries, binary)
	}
	if info, err := os.Stat(filepath.Join(goroot, "src", "runtime")); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("matching Go runtime source tree is required")
	}
	files, goNotices, err := collect(goroot, "go", "GOROOT", false, func(rel string) bool {
		return rel == "pkg" || rel == "bin" || rel == "src/cmd" || rel == "test" || rel == "doc" || rel == "misc" || rel == "lib"
	})
	if err != nil {
		return nil, err
	}
	goNotices, err = goRootNotices(repository, goVersion, files, goNotices)
	if err != nil {
		return nil, err
	}
	inventory.Components = append(inventory.Components, Component{Module{"go", goVersion}, "https://go.dev/dl/#" + goVersion, goNotices})
	paths := make([]string, 0, len(modules))
	for module := range modules {
		paths = append(paths, module)
	}
	slices.Sort(paths)
	for _, module := range paths {
		origin := "vendor/" + module
		moduleFiles, notices, err := collect(filepath.Join(repository, filepath.FromSlash(origin)), "modules/"+module, origin, true, func(rel string) bool {
			_, nested := versions[module+"/"+rel]
			return nested
		})
		if err != nil {
			return nil, err
		}
		for name, data := range moduleFiles {
			files[name] = data
		}
		inventory.Components = append(inventory.Components, Component{Module{module, modules[module]}, "https://pkg.go.dev/" + module + "@" + modules[module], notices})
	}
	encoded, err := json.MarshalIndent(inventory, "", "  ")
	if err != nil {
		return nil, err
	}
	files["inventory.json"] = append(encoded, '\n')
	files["README.txt"] = []byte("Third-party notices for the accompanying skopeo.exe and store-launcher.exe.\n\n" +
		"inventory.json records their SHA-256 hashes, Go versions, linked module versions, notice source paths and file hashes.\n" +
		"Module notice files are copied verbatim from version-matched vendored sources; SOURCE-COPYRIGHTS.txt preserves leading source copyright/license comments.\n" +
		"Go notices include a conservative superset from the matching Go source tree, excluding compiler tools and test data.\n" +
		"Components and subcomponents retain their respective licenses; the project Apache License does not replace them.\n\n" +
		"Source checkout: https://github.com/rambow-cloud/skopeo-win (vendor/ and vendor/modules.txt).\n" +
		"Versioned dependency source links: https://pkg.go.dev/<module-path>@<version> using inventory.json.\n" +
		"Go source: https://go.dev/dl/ using the recorded exact Go version.\n")
	return files, nil
}

// Run generates or verifies notices against the actual binaries and local sources.
// Verification recomputes the inventory, rather than trusting packaged hashes.
func Run(repository, goroot, layout string, verify bool) error {
	inputs, err := readInputs(layout)
	if err != nil {
		return err
	}
	files, err := buildFiles(repository, goroot, inputs)
	if err != nil {
		return err
	}
	root := filepath.Join(layout, Directory)
	if verify {
		return verifyFiles(root, files)
	}
	if err := os.Mkdir(root, 0o755); err != nil {
		return err
	}
	for name, data := range files {
		file := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(file, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func verifyFiles(root string, expected map[string][]byte) error {
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("missing or invalid %s directory", Directory)
	}
	seen := map[string]bool{}
	err = filepath.WalkDir(root, func(file string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, file)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		data, exists := expected[rel]
		if !exists || !entry.Type().IsRegular() {
			return fmt.Errorf("unexpected notice file %s", rel)
		}
		actual, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if !bytes.Equal(data, actual) {
			return fmt.Errorf("notice content differs from matching sources: %s", rel)
		}
		seen[rel] = true
		return nil
	})
	if err != nil {
		return err
	}
	for name := range expected {
		if !seen[name] {
			return fmt.Errorf("missing notice file %s", name)
		}
	}
	return nil
}
