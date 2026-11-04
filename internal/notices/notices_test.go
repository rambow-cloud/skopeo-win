package notices

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
)

func writeFixture(t *testing.T, root, name string, data []byte) {
	t.Helper()
	file := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func fixture(t *testing.T) (string, string, []input) {
	t.Helper()
	repository, goroot := t.TempDir(), t.TempDir()
	writeFixture(t, repository, "vendor/modules.txt", []byte("# example.test/runtime v1.2.3\n# example.test/runtime/child v2.0.0\n# example.test/unused v1.0.0\n"))
	for _, module := range []string{"runtime", "runtime/child", "unused"} {
		writeFixture(t, repository, "vendor/example.test/"+module+"/LICENSE", []byte("Fixture license for "+module+"\r\n"))
	}
	writeFixture(t, repository, "vendor/example.test/runtime/NOTICE", []byte("Copyright fixture author. Preserve this notice.\r\n"))
	writeFixture(t, repository, "vendor/example.test/runtime/package.go", []byte("// Copyright fixture source author.\r\n// Full license paragraph must survive.\r\n\r\npackage fixture\n"))
	writeFixture(t, repository, "vendor/example.test/runtime/sub/assembly.s", []byte("/* Copyright assembly author.\n   Distinct assembly license terms. */\n#include \"textflag.h\"\n"))
	writeFixture(t, goroot, "VERSION", []byte("go1.27.3\ntime 2026-10-20\n"))
	writeFixture(t, goroot, "LICENSE", []byte("Fixture Go runtime license\n"))
	writeFixture(t, goroot, "PATENTS", []byte("Fixture Go patent grant\n"))
	writeFixture(t, goroot, "src/runtime/fixture.go", []byte("// Copyright fixture runtime author.\npackage runtime\n"))
	writeFixture(t, goroot, "src/cmd/unused/LICENSE", []byte("Compiler tools are not shipped\n"))
	inputs := []input{
		{binary: Binary{File: "skopeo.exe", SHA256: "fixture-cli", GoVersion: "go1.27.3", GOARCH: "amd64"}, info: &debug.BuildInfo{GoVersion: "go1.27.3", Deps: []*debug.Module{{Path: "example.test/runtime", Version: "v1.2.3"}, {Path: "example.test/runtime/child", Version: "v2.0.0"}}}},
		{binary: Binary{File: "store-launcher.exe", SHA256: "fixture-launcher", GoVersion: "go1.27.3", GOARCH: "amd64"}, info: &debug.BuildInfo{GoVersion: "go1.27.3"}},
	}
	return repository, goroot, inputs
}

func TestExactNoticeSources(t *testing.T) {
	repository, goroot, inputs := fixture(t)
	files, err := buildFiles(repository, goroot, inputs)
	if err != nil {
		t.Fatal(err)
	}
	if string(files["modules/example.test/runtime/NOTICE"]) != "Copyright fixture author. Preserve this notice.\r\n" {
		t.Fatal("notice bytes changed")
	}
	copyrights := files["modules/example.test/runtime/SOURCE-COPYRIGHTS.txt"]
	if !bytes.Contains(copyrights, []byte("// Copyright fixture source author.\r\n// Full license paragraph must survive.\r\n")) ||
		!bytes.Contains(copyrights, []byte("/* Copyright assembly author.\n   Distinct assembly license terms. */")) {
		t.Fatal("source copyright or complete license header was omitted")
	}
	for name := range files {
		if strings.Contains(name, "example.test/unused") || strings.Contains(name, "/src/cmd/") || strings.Contains(name, "/runtime/child/child/") {
			t.Fatalf("unrelated component attributed to shipped dependencies: %s", name)
		}
	}
	var inventory Inventory
	if err := json.Unmarshal(files["inventory.json"], &inventory); err != nil {
		t.Fatal(err)
	}
	if len(inventory.Components) != 3 || len(inventory.Binaries[0].Modules) != 2 || len(inventory.Binaries[1].Modules) != 0 {
		t.Fatalf("wrong linked dependency inventory: %+v", inventory)
	}
	for _, component := range inventory.Components {
		for _, notice := range component.Notices {
			if notice.SHA256 != digest(files[notice.File]) || notice.Source == "" {
				t.Fatalf("missing source attribution or wrong notice digest: %+v", notice)
			}
			if component.Path == "example.test/runtime" && strings.Contains(notice.File, "/child/") {
				t.Fatal("nested module was attributed to parent")
			}
		}
	}
	second, err := buildFiles(repository, goroot, inputs)
	if err != nil || !bytes.Equal(files["inventory.json"], second["inventory.json"]) {
		t.Fatal("inventory is not deterministic")
	}
}

func TestHistoricalSourcesFailClosed(t *testing.T) {
	for _, scenario := range []string{"old Go", "old dependency", "replacement", "missing module", "missing license", "empty Go version", "vendor replacement", "copyright filename collision"} {
		t.Run(scenario, func(t *testing.T) {
			repository, goroot, inputs := fixture(t)
			switch scenario {
			case "old Go":
				inputs[0].info.GoVersion = "go1.25.6"
			case "old dependency":
				inputs[0].info.Deps[0].Version = "v1.1.0"
			case "replacement":
				inputs[0].info.Deps[0].Replace = &debug.Module{Path: "example.test/replacement", Version: "v1.2.3"}
			case "missing module":
				inputs[0].info.Deps[0].Path = "example.test/missing"
			case "missing license":
				if err := os.Remove(filepath.Join(repository, "vendor", "example.test", "runtime", "LICENSE")); err != nil {
					t.Fatal(err)
				}
			case "empty Go version":
				writeFixture(t, goroot, "VERSION", nil)
			case "vendor replacement":
				writeFixture(t, repository, "vendor/modules.txt", []byte("# example.test/runtime v1.2.3 => example.test/fork v1.2.3\n"))
			case "copyright filename collision":
				writeFixture(t, repository, "vendor/example.test/runtime/SOURCE-COPYRIGHTS.txt", []byte("Original notice must not be overwritten\n"))
			}
			if _, err := buildFiles(repository, goroot, inputs); err == nil {
				t.Fatal("unmatched historical sources were accepted")
			}
		})
	}
}

func TestTamperedPackageNoticesRejected(t *testing.T) {
	repository, goroot, inputs := fixture(t)
	expected, err := buildFiles(repository, goroot, inputs)
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"valid", "missing license", "changed notice", "missing copyrights", "wrong module version", "wrong executable hash", "extra file"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			for name, data := range expected {
				writeFixture(t, root, name, data)
			}
			switch scenario {
			case "missing license":
				if err := os.Remove(filepath.Join(root, "modules", "example.test", "runtime", "LICENSE")); err != nil {
					t.Fatal(err)
				}
			case "changed notice":
				writeFixture(t, root, "modules/example.test/runtime/NOTICE", []byte("Changed copyright attribution"))
			case "missing copyrights":
				if err := os.Remove(filepath.Join(root, "modules", "example.test", "runtime", "SOURCE-COPYRIGHTS.txt")); err != nil {
					t.Fatal(err)
				}
			case "wrong module version":
				writeFixture(t, root, "inventory.json", bytes.ReplaceAll(expected["inventory.json"], []byte("v1.2.3"), []byte("v1.1.0")))
			case "wrong executable hash":
				writeFixture(t, root, "inventory.json", bytes.ReplaceAll(expected["inventory.json"], []byte("fixture-cli"), []byte("different-cli")))
			case "extra file":
				writeFixture(t, root, "unattributed-LICENSE.txt", []byte("Unattributed content"))
			}
			err := verifyFiles(root, expected)
			if (err == nil) != (scenario == "valid") {
				t.Fatalf("unexpected package verification for %s: %v", scenario, err)
			}
		})
	}
}

func TestPinnedGoRootNotices(t *testing.T) {
	for _, scenario := range []string{"valid", "wrong version", "changed snapshot", "missing snapshot", "missing runtime sources"} {
		t.Run(scenario, func(t *testing.T) {
			repository, goroot, inputs := fixture(t)
			for _, name := range []string{"LICENSE", "PATENTS"} {
				if err := os.Remove(filepath.Join(goroot, name)); err != nil {
					t.Fatal(err)
				}
			}
			origin := "packaging/store/go-notices/go1.27.3/"
			license, patents := []byte("Pinned fixture runtime license\n"), []byte("Pinned fixture patent grant\n")
			writeFixture(t, repository, origin+"LICENSE", license)
			writeFixture(t, repository, origin+"PATENTS", patents)
			metadata := map[string]any{
				"goVersion": "go1.27.3", "repository": "https://github.com/golang/go", "commit": strings.Repeat("a", 40),
				"sha256": map[string]string{"LICENSE": digest(license), "PATENTS": digest(patents)},
			}
			if scenario == "wrong version" {
				metadata["goVersion"] = "go1.25.6"
			}
			encoded, err := json.Marshal(metadata)
			if err != nil {
				t.Fatal(err)
			}
			if scenario != "missing snapshot" {
				writeFixture(t, repository, origin+"source.json", encoded)
			}
			if scenario == "changed snapshot" {
				writeFixture(t, repository, origin+"LICENSE", []byte("Altered pinned license"))
			}
			if scenario == "missing runtime sources" {
				if err := os.Remove(filepath.Join(goroot, "src", "runtime", "fixture.go")); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(filepath.Join(goroot, "src", "runtime")); err != nil {
					t.Fatal(err)
				}
			}
			files, err := buildFiles(repository, goroot, inputs)
			if (err == nil) != (scenario == "valid") {
				t.Fatalf("incorrect pinned snapshot acceptance for %s: %v", scenario, err)
			}
			if scenario == "valid" && (!bytes.Equal(files["go/LICENSE"], license) || !bytes.Contains(files["inventory.json"], []byte("https://github.com/golang/go/blob/"+strings.Repeat("a", 40)+"/LICENSE"))) {
				t.Fatal("pinned license bytes or immutable origin were not retained")
			}
		})
	}
}
