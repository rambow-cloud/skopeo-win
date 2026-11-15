package notices

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
)

func reviewedFixture(t *testing.T) (string, string, []input, ReviewedSource) {
	t.Helper()
	repository, goroot, inputs := fixture(t)
	source := ReviewedSource{
		Module: "example.test/runtime", Version: "v1.2.3",
		Replacement: Module{Path: "github.com/example/fork/runtime", Version: "v1.2.3-reviewed.1"},
		Repository: "https://github.com/example/fork", Commit: strings.Repeat("a", 40), Directory: "runtime",
		UpstreamRepository: "https://github.com/example/upstream", UpstreamCommit: strings.Repeat("b", 40),
	}
	var err error
	source.VendorSHA256, err = SourceDigest(filepath.Join(repository, "vendor", "example.test", "runtime"))
	if err != nil { t.Fatal(err) }
	data, err := json.Marshal([]ReviewedSource{source})
	if err != nil { t.Fatal(err) }
	writeFixture(t, repository, "packaging/store/module-sources.json", data)
	writeFixture(t, repository, "vendor/modules.txt", []byte("# example.test/runtime v1.2.3 => github.com/example/fork/runtime v1.2.3-reviewed.1\n# example.test/runtime/child v2.0.0\n# example.test/unused v1.0.0\n# example.test/runtime => github.com/example/fork/runtime v1.2.3-reviewed.1\n"))
	inputs[0].info.Deps[0].Replace = &debug.Module{Path: source.Replacement.Path, Version: source.Replacement.Version}
	return repository, goroot, inputs, source
}

func TestReviewedReplacementNoticeProvenance(t *testing.T) {
	repository, goroot, inputs, source := reviewedFixture(t)
	files, err := buildFiles(repository, goroot, inputs)
	if err != nil { t.Fatal(err) }
	var inventory Inventory
	if err := json.Unmarshal(files["inventory.json"], &inventory); err != nil { t.Fatal(err) }
	seen := false
	for _, component := range inventory.Components {
		if component.Path != source.Module { continue }
		seen = true
		if component.Replacement == nil || *component.Replacement != source.Replacement ||
			component.SourceURL != source.Repository+"/tree/"+source.Commit+"/runtime" ||
			component.UpstreamSourceURL != source.UpstreamRepository+"/tree/"+source.UpstreamCommit+"/runtime" ||
			component.VendorSourceSHA256 != source.VendorSHA256 {
			t.Fatalf("missing exact fork provenance: %+v", component)
		}
	}
	if !seen || inventory.Binaries[0].Modules[0].Replacement == nil { t.Fatal("binary fork provenance was omitted") }
}

func TestReviewedReplacementMismatchRejected(t *testing.T) {
	for _, scenario := range []string{"missing policy", "changed source", "extra source", "missing source", "wrong binary fork", "wrong binary version", "canonical binary", "wrong vendor fork", "wrong footer", "missing replacement", "invalid provenance", "duplicate record"} {
		t.Run(scenario, func(t *testing.T) {
			repository, goroot, inputs, source := reviewedFixture(t)
			switch scenario {
			case "missing policy":
				if err := os.Remove(filepath.Join(repository,"packaging","store","module-sources.json")); err != nil { t.Fatal(err) }
			case "changed source":
				writeFixture(t, repository, "vendor/example.test/runtime/package.go", []byte("package changed\n"))
			case "extra source":
				writeFixture(t, repository, "vendor/example.test/runtime/extra.go", []byte("package fixture\n"))
			case "missing source":
				if err := os.Remove(filepath.Join(repository,"vendor","example.test","runtime","package.go")); err != nil { t.Fatal(err) }
			case "wrong binary fork": inputs[0].info.Deps[0].Replace.Path = "github.com/example/unreviewed/runtime"
			case "wrong binary version": inputs[0].info.Deps[0].Replace.Version = "v1.2.3-reviewed.2"
			case "canonical binary": inputs[0].info.Deps[0].Replace = nil
			case "wrong vendor fork":
				writeFixture(t, repository, "vendor/modules.txt", []byte("# example.test/runtime v1.2.3 => github.com/example/other/runtime v1.2.3-reviewed.1\n"))
			case "wrong footer":
				writeFixture(t, repository, "vendor/modules.txt", []byte("# example.test/runtime v1.2.3 => github.com/example/fork/runtime v1.2.3-reviewed.1\n# example.test/runtime => github.com/example/other/runtime v1.2.3-reviewed.1\n"))
			case "missing replacement":
				writeFixture(t, repository, "vendor/modules.txt", []byte("# example.test/runtime v1.2.3\n"))
			case "invalid provenance":
				source.Commit = "not-a-commit"
				data, _ := json.Marshal([]ReviewedSource{source})
				writeFixture(t, repository, "packaging/store/module-sources.json", data)
			case "duplicate record":
				data, _ := json.Marshal([]ReviewedSource{source,source})
				writeFixture(t, repository, "packaging/store/module-sources.json", data)
			}
			if _, err := buildFiles(repository, goroot, inputs); err == nil { t.Fatal("unreviewed or mismatched sources were accepted") }
		})
	}
}
