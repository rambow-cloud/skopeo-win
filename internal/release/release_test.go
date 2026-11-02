package release

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const testCommit = "1111111111111111111111111111111111111111"

func ref(tag, commit string) Ref {
	r := Ref{Ref: "refs/tags/" + tag}
	r.Object.Type, r.Object.SHA = "commit", commit
	return r
}

func TestNext(t *testing.T) {
	for _, tc := range []struct {
		name string
		tags []string
		want string
	}{
		{"first independent release", []string{LegacyTag}, "1.0.0"},
		{"increment", []string{LegacyTag, "v1.0.0"}, "1.0.1"},
		{"numeric sorting and reserved tags", []string{"v1.0.2", "v1.0.10", "v9.0.0-rc.1"}, "1.0.11"},
		{"skip legacy collision", []string{"v1.22.9", "v1.23.0", "v1.23.0-rc.1"}, "1.22.10"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var refs []Ref
			for _, tag := range tc.tags {
				refs = append(refs, ref(tag, testCommit))
			}
			got, err := Next(refs, nil, "123", testCommit)
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v; want %s", got, err, tc.want)
			}
		})
	}
	t.Run("draft reserves next number", func(t *testing.T) {
		got, err := Next(nil, []Release{{Tag: "v1.0.1", Draft: true}}, "123", testCommit)
		if err != nil || got != "1.0.2" {
			t.Fatalf("got %q, %v", got, err)
		}
	})
	t.Run("same run reuse", func(t *testing.T) {
		got, err := Next([]Ref{ref("v1.0.0", testCommit)}, []Release{{Tag: "v1.0.0", Body: marker("123")}}, "123", testCommit)
		if err != nil || got != "1.0.0" {
			t.Fatalf("got %q, %v", got, err)
		}
	})
	t.Run("same run cannot move tag", func(t *testing.T) {
		_, err := Next([]Ref{ref("v1.0.0", "other")}, []Release{{Tag: "v1.0.0", Body: marker("123")}}, "123", testCommit)
		if err == nil {
			t.Fatal("expected tag collision error")
		}
	})
}

type fakeGH struct {
	refs      []Ref
	releases  []Release
	expected  map[string]string
	mutations []string
	failAsset string
}

func (f *fakeGH) run(args ...string) ([]byte, error) {
	if args[0] == "api" && args[1] == "--method" {
		f.mutations = append(f.mutations, "create-tag")
		f.refs = append(f.refs, ref(strings.TrimPrefix(args[5], "ref=refs/tags/"), testCommit))
		return nil, nil
	}
	if args[0] == "api" {
		if strings.Contains(args[1], "matching-refs") {
			return json.Marshal([][]Ref{f.refs})
		}
		return json.Marshal([][]Release{f.releases})
	}
	f.mutations = append(f.mutations, args[1])
	switch args[1] {
	case "create":
		f.releases = append(f.releases, Release{Tag: args[2], Draft: true, Body: marker("123")})
	case "upload":
		name := filepath.Base(args[3])
		if name == f.failAsset {
			return nil, fmt.Errorf("simulated upload interruption")
		}
		f.releases[0].Assets = append(f.releases[0].Assets, Asset{Name: name, Digest: f.expected[name]})
	case "edit":
		f.releases[0].Draft = false
	default:
		return nil, fmt.Errorf("unexpected mutation %v", args)
	}
	return nil, nil
}

func fixture(t *testing.T) (string, map[string]string) {
	t.Helper()
	dir := t.TempDir()
	for _, arch := range []string{"amd64", "arm64"} {
		name := "skopeo-windows-" + arch + ".exe"
		data := []byte(arch)
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name+".sha256"), []byte(fmt.Sprintf("%x  %s\n", sha256.Sum256(data), name)), 0600); err != nil {
			t.Fatal(err)
		}
	}
	expected, err := Files(dir)
	if err != nil {
		t.Fatal(err)
	}
	return dir, expected
}

func TestPublishAndRetry(t *testing.T) {
	dir, expected := fixture(t)
	f := &fakeGH{expected: expected}
	m := Manager{Run: f.run, Repo: "owner/repo", Commit: testCommit, RunID: "123"}
	if err := m.Publish("1.0.0", dir); err != nil {
		t.Fatal(err)
	}
	if f.releases[0].Draft || len(f.releases[0].Assets) != 4 {
		t.Fatal("did not publish all four assets")
	}
	f.mutations = nil
	if err := m.Publish("1.0.0", dir); err != nil {
		t.Fatal(err)
	}
	if len(f.mutations) != 0 {
		t.Fatal("published release was mutated on retry")
	}
}

func TestInterruptedUploadResumesWithoutReplacingAssets(t *testing.T) {
	dir, expected := fixture(t)
	f := &fakeGH{expected: expected, failAsset: "skopeo-windows-arm64.exe"}
	m := Manager{Run: f.run, Repo: "owner/repo", Commit: testCommit, RunID: "123"}
	if err := m.Publish("1.0.0", dir); err == nil {
		t.Fatal("expected upload failure")
	}
	if !f.releases[0].Draft || len(f.releases[0].Assets) != 2 {
		t.Fatal("partial release must remain a draft")
	}
	f.failAsset, f.mutations = "", nil
	if err := m.Publish("1.0.0", dir); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.mutations, []string{"upload", "upload", "edit"}) {
		t.Fatalf("unexpected retry mutations %v", f.mutations)
	}
}

func TestRejectUnsafePublication(t *testing.T) {
	for _, tc := range []string{"tag collision", "foreign run", "mismatched published asset", "missing published asset", "corrupt checksum", "legacy version", "unexpected file"} {
		t.Run(tc, func(t *testing.T) {
			dir, expected := fixture(t)
			f := &fakeGH{expected: expected, refs: []Ref{ref("v1.0.0", testCommit)}, releases: []Release{{Tag: "v1.0.0", Body: marker("123")}}}
			for name, digest := range expected {
				f.releases[0].Assets = append(f.releases[0].Assets, Asset{Name: name, Digest: digest})
			}
			version := "1.0.0"
			switch tc {
			case "tag collision":
				f.refs[0].Object.SHA = "other"
			case "foreign run":
				f.releases[0].Body = marker("999")
			case "mismatched published asset":
				f.releases[0].Assets[0].Digest = "sha256:wrong"
			case "missing published asset":
				f.releases[0].Assets = f.releases[0].Assets[:3]
			case "corrupt checksum":
				if err := os.WriteFile(filepath.Join(dir, "skopeo-windows-amd64.exe"), []byte("changed"), 0600); err != nil {
					t.Fatal(err)
				}
			case "legacy version":
				version = "1.23.0"
			case "unexpected file":
				if err := os.WriteFile(filepath.Join(dir, "extra"), nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			m := Manager{Run: f.run, Repo: "owner/repo", Commit: testCommit, RunID: "123"}
			if err := m.Publish(version, dir); err == nil {
				t.Fatal("expected publication refusal")
			}
			if len(f.mutations) != 0 {
				t.Fatalf("unsafe mutation: %v", f.mutations)
			}
		})
	}
}
