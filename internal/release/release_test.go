package release

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const testCommit = "1111111111111111111111111111111111111111"

func testRef(tag, commit string) Ref {
	r := Ref{Ref: "refs/tags/" + tag}
	r.Object.Type, r.Object.SHA = "commit", commit
	return r
}

// releaseStore models independent service constraints: unique refs/releases,
// immutable public assets, content-derived digests and publication by ID.
// It rejects unknown commands rather than treating them as successful reads.
type releaseStore struct {
	refs           map[string]string
	releases       map[int64]*Release
	mutations      []string
	snapshots      int
	beforeSnapshot func(int)
	failAsset      string
	badCreate      bool
}

func newStore() *releaseStore {
	return &releaseStore{refs: map[string]string{"v1.0.0": testCommit}, releases: map[int64]*Release{}}
}

func apiForm(args []string) (string, string, map[string]string, error) {
	method, path := "", ""
	fields := map[string]string{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--method", "-f", "-F":
			option := args[i]
			i++
			if i >= len(args) {
				return "", "", nil, fmt.Errorf("missing option value")
			}
			if option == "--method" {
				method = args[i]
				continue
			}
			key, value, ok := strings.Cut(args[i], "=")
			if !ok {
				return "", "", nil, fmt.Errorf("invalid form field")
			}
			if _, exists := fields[key]; exists {
				return "", "", nil, fmt.Errorf("duplicate form field")
			}
			fields[key] = value
		default:
			if strings.HasPrefix(args[i], "-") || path != "" {
				return "", "", nil, fmt.Errorf("unexpected API argument %q", args[i])
			}
			path = args[i]
		}
	}
	return method, path, fields, nil
}

func (s *releaseStore) run(args ...string) ([]byte, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("empty command")
	}
	if len(args) == 4 && args[0] == "api" && args[1] == "repos/owner/repo/compare/"+testCommit+"...main" && args[2] == "--jq" && args[3] == ".merge_base_commit.sha" {
		return []byte(testCommit), nil
	}
	if args[0] == "api" && len(args) == 4 && args[2] == "--paginate" && args[3] == "--slurp" {
		switch args[1] {
		case "repos/owner/repo/git/matching-refs/tags/v":
			s.snapshots++
			if s.beforeSnapshot != nil {
				s.beforeSnapshot(s.snapshots)
			}
			var refs []Ref
			for tag, commit := range s.refs {
				refs = append(refs, testRef(tag, commit))
			}
			return json.Marshal([][]Ref{refs})
		case "repos/owner/repo/releases":
			var releases []Release
			for _, r := range s.releases {
				releases = append(releases, *r)
			}
			return json.Marshal([][]Release{releases})
		default:
			return nil, fmt.Errorf("unexpected read API %v", args)
		}
	}
	if args[0] == "api" {
		method, path, fields, err := apiForm(args[1:])
		if err != nil {
			return nil, err
		}
		s.mutations = append(s.mutations, method+" "+path)
		switch {
		case method == "POST" && path == "repos/owner/repo/releases":
			tag := fields["tag_name"]
			if len(fields) != 5 || fields["draft"] != "true" || s.refs[tag] != fields["target_commitish"] || fields["name"] != "skopeo-win "+tag {
				return nil, fmt.Errorf("invalid draft creation")
			}
			for _, r := range s.releases {
				if r.Tag == tag {
					return nil, fmt.Errorf("release already exists")
				}
			}
			id := int64(100)
			for existing := range s.releases {
				if existing >= id {
					id = existing + 1
				}
			}
			r := &Release{ID: id, Tag: tag, Body: fields["body"], Draft: true}
			s.releases[r.ID] = r
			if s.badCreate {
				return []byte(`{"id":0,"tag_name":"wrong","draft":false}`), nil
			}
			return json.Marshal(r)
		case method == "PATCH" && strings.HasPrefix(path, "repos/owner/repo/releases/"):
			id, err := strconv.ParseInt(strings.TrimPrefix(path, "repos/owner/repo/releases/"), 10, 64)
			if err != nil {
				return nil, err
			}
			r := s.releases[id]
			if r == nil || !r.Draft || len(fields) != 2 || fields["draft"] != "false" || fields["make_latest"] != "true" {
				return nil, fmt.Errorf("unsafe publication request")
			}
			if len(r.Assets) != 4 {
				return nil, fmt.Errorf("partial release cannot be published")
			}
			r.Draft = false
			return json.Marshal(r)
		default:
			return nil, fmt.Errorf("unexpected write API %v", args)
		}
	}
	if len(args) == 6 && args[0] == "release" && args[1] == "upload" && args[4] == "--repo" && args[5] == "owner/repo" {
		name := filepath.Base(args[3])
		s.mutations = append(s.mutations, "upload "+name)
		if name == s.failAsset {
			return nil, fmt.Errorf("interrupted upload")
		}
		for _, r := range s.releases {
			if r.Tag != args[2] {
				continue
			}
			if !r.Draft {
				return nil, fmt.Errorf("public release is immutable")
			}
			for _, a := range r.Assets {
				if a.Name == name {
					return nil, fmt.Errorf("asset replacement forbidden")
				}
			}
			b, err := os.ReadFile(args[3])
			if err != nil {
				return nil, err
			}
			r.Assets = append(r.Assets, Asset{Name: name, Digest: fmt.Sprintf("sha256:%x", sha256.Sum256(b))})
			return nil, nil
		}
		return nil, fmt.Errorf("release does not exist")
	}
	return nil, fmt.Errorf("unsupported command %v", args)
}

func releaseFixture(t *testing.T) (string, map[string]string) {
	t.Helper()
	dir := t.TempDir()
	expected := map[string]string{}
	for _, arch := range []string{"amd64", "arm64"} {
		name := "skopeo-windows-" + arch + ".exe"
		b := []byte("unit fixture " + arch)
		checksum := []byte(fmt.Sprintf("%x  %s\n", sha256.Sum256(b), name))
		for n, data := range map[string][]byte{name: b, name + ".sha256": checksum} {
			if err := os.WriteFile(filepath.Join(dir, n), data, 0o600); err != nil {
				t.Fatal(err)
			}
			expected[n] = fmt.Sprintf("sha256:%x", sha256.Sum256(data))
		}
	}
	return dir, expected
}

func manager(s *releaseStore) Manager {
	return Manager{Run: s.run, Repo: "owner/repo", Commit: testCommit, RunID: "123", Summary: "## Features\n\nReviewed feature description."}
}

func TestPublicationAndReadOnlyRetry(t *testing.T) {
	dir, expected := releaseFixture(t)
	s := newStore()
	m := manager(s)
	if err := m.Publish("1.0.0", dir); err != nil {
		t.Fatal(err)
	}
	r := s.releases[100]
	if r == nil || r.Draft || s.refs["v1.0.0"] != testCommit || len(r.Assets) != 4 {
		t.Fatal("publication was incomplete")
	}
	for _, a := range r.Assets {
		if expected[a.Name] != a.Digest {
			t.Fatalf("asset bytes differ: %s", a.Name)
		}
	}
	if !strings.Contains(r.Body, "Reviewed feature description.") || !strings.Contains(r.Body, marker("123")) {
		t.Fatal("notes lost reviewed content or ownership")
	}
	if s.mutations[len(s.mutations)-1] != "PATCH repos/owner/repo/releases/100" {
		t.Fatal("publication was not bound to the draft ID")
	}
	s.mutations = nil
	if err := m.Publish("1.0.0", dir); err != nil {
		t.Fatal(err)
	}
	if len(s.mutations) != 0 {
		t.Fatal("published retry mutated state")
	}
}

func TestInterruptedUploadOnlyResumesMissingBytes(t *testing.T) {
	dir, _ := releaseFixture(t)
	s := newStore()
	m := manager(s)
	s.failAsset = "skopeo-windows-arm64.exe"
	if err := m.Publish("1.0.0", dir); err == nil {
		t.Fatal("expected interrupted upload")
	}
	if !s.releases[100].Draft || len(s.releases[100].Assets) != 2 {
		t.Fatal("partial release escaped draft state")
	}
	s.failAsset = ""
	s.mutations = nil
	if err := m.Publish("1.0.0", dir); err != nil {
		t.Fatal(err)
	}
	want := []string{"upload skopeo-windows-arm64.exe", "upload skopeo-windows-arm64.exe.sha256", "PATCH repos/owner/repo/releases/100"}
	if strings.Join(s.mutations, "\n") != strings.Join(want, "\n") {
		t.Fatalf("retry replaced existing state: %v", s.mutations)
	}
}

func TestPublicationRevalidatesRemoteStateAfterUploads(t *testing.T) {
	for _, change := range []string{"ownership", "replacement ID", "tag moved", "asset modified", "already public"} {
		t.Run(change, func(t *testing.T) {
			dir, _ := releaseFixture(t)
			s := newStore()
			m := manager(s)
			s.beforeSnapshot = func(n int) {
				if n != 2 {
					return
				}
				r := s.releases[100]
				switch change {
				case "ownership":
					r.Body = marker("999")
				case "replacement ID":
					copy := *r
					copy.ID = 200
					delete(s.releases, 100)
					s.releases[200] = &copy
				case "tag moved":
					s.refs["v1.0.0"] = "different commit"
				case "asset modified":
					r.Assets[0].Digest = "sha256:wrong"
				case "already public":
					r.Draft = false
				}
			}
			err := m.Publish("1.0.0", dir)
			if change == "already public" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("changed state was accepted")
			}
			for _, mutation := range s.mutations {
				if strings.HasPrefix(mutation, "PATCH ") {
					t.Fatalf("changed/public release was edited: %v", s.mutations)
				}
			}
		})
	}
}

func TestUnsafeInitialStateIsRejectedBeforeMutation(t *testing.T) {
	for _, change := range []string{"foreign run", "missing identity", "wrong tag", "missing asset", "wrong asset", "checksum", "unexpected file", "legacy"} {
		t.Run(change, func(t *testing.T) {
			dir, expected := releaseFixture(t)
			s := newStore()
			m := manager(s)
			r := &Release{ID: 100, Tag: "v1.0.0", Body: marker("123"), Draft: false}
			for name, digest := range expected {
				r.Assets = append(r.Assets, Asset{Name: name, Digest: digest})
			}
			s.releases[r.ID] = r
			s.refs[r.Tag] = testCommit
			version := "1.0.0"
			switch change {
			case "foreign run":
				r.Body = marker("456")
			case "missing identity":
				r.ID = 0
			case "wrong tag":
				s.refs[r.Tag] = "other"
			case "missing asset":
				r.Assets = r.Assets[:3]
			case "wrong asset":
				r.Assets[0].Digest = "sha256:wrong"
			case "checksum":
				if err := os.WriteFile(filepath.Join(dir, "skopeo-windows-amd64.exe"), []byte("changed"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "unexpected file":
				if err := os.WriteFile(filepath.Join(dir, "extra"), nil, 0o600); err != nil {
					t.Fatal(err)
				}
			case "legacy":
				version = "1.23.0"
			}
			if err := m.Publish(version, dir); err == nil {
				t.Fatal("unsafe state accepted")
			}
			if len(s.mutations) != 0 {
				t.Fatalf("unsafe state mutated: %v", s.mutations)
			}
		})
	}
}

func TestMalformedDraftResponseStopsBeforeUpload(t *testing.T) {
	dir, _ := releaseFixture(t)
	s := newStore()
	s.badCreate = true
	if err := manager(s).Publish("1.0.0", dir); err == nil {
		t.Fatal("malformed draft response accepted")
	}
	for _, mutation := range s.mutations {
		if strings.HasPrefix(mutation, "upload ") || strings.HasPrefix(mutation, "PATCH ") {
			t.Fatal("invalid draft received assets or publication")
		}
	}
}

func TestManualTagPlanningAndPublication(t *testing.T) {
	for _, state := range []string{"new lightweight tag", "annotated tag", "same-run retry", "missing tag", "moved tag", "outside main", "foreign release", "newer orphan", "newer draft"} {
		t.Run(state, func(t *testing.T) {
			s := newStore()
			s.refs[LegacyTag] = testCommit
			s.refs["v1.0.15"] = testCommit
			m := manager(s)
			switch state {
			case "annotated tag":
				m.Run = func(args ...string) ([]byte, error) {
					if len(args) == 4 && args[1] == "repos/owner/repo/commits/v1.0.15" {
						return []byte(testCommit), nil
					}
					data, err := s.run(args...)
					if len(args) == 4 && strings.HasSuffix(args[1], "/git/matching-refs/tags/v") {
						var pages [][]Ref
						if err := json.Unmarshal(data, &pages); err != nil {
							return nil, err
						}
						for i := range pages[0] {
							if pages[0][i].Ref == "refs/tags/v1.0.15" {
								pages[0][i].Object.Type = "tag"
								pages[0][i].Object.SHA = strings.Repeat("2", 40)
							}
						}
						return json.Marshal(pages)
					}
					return data, err
				}
			case "same-run retry":
				s.releases[100] = &Release{ID: 100, Tag: "v1.0.15", Draft: true, Body: marker("123")}
			case "missing tag":
				delete(s.refs, "v1.0.15")
			case "moved tag":
				s.refs["v1.0.15"] = "wrong"
			case "outside main":
				m.Run = func(args ...string) ([]byte, error) {
					if len(args) == 4 && strings.Contains(args[1], "/compare/") {
						return []byte(strings.Repeat("2", 40)), nil
					}
					return s.run(args...)
				}
			case "foreign release":
				s.releases[100] = &Release{ID: 100, Tag: "v1.0.15", Body: marker("999")}
			case "newer orphan":
				s.refs["v1.0.16"] = testCommit
			case "newer draft":
				s.releases[100] = &Release{ID: 100, Tag: "v1.0.16", Draft: true}
			}
			got, err := m.PlanTag("v1.0.15")
			good := state == "new lightweight tag" || state == "annotated tag" || state == "same-run retry"
			if good && (err != nil || got != "1.0.15") || !good && err == nil {
				t.Fatalf("unexpected plan %q: %v", got, err)
			}
			if len(s.mutations) != 0 {
				t.Fatal("planning changed remote state")
			}
			dir, _ := releaseFixture(t)
			err = m.Publish("1.0.15", dir)
			if good && err != nil || !good && err == nil {
				t.Fatalf("unexpected publication: %v", err)
			}
			if !good && len(s.mutations) != 0 {
				t.Fatal("rejected release changed remote state")
			}
			for _, mutation := range s.mutations {
				if strings.Contains(mutation, "git/refs") {
					t.Fatal("publisher created or moved a tag")
				}
			}
		})
	}
}

func TestManualTagSyntaxAndVersionOrder(t *testing.T) {
	for _, tag := range []string{"", "1.0.15", "v01.0.15", "v0.1.0", "v1.0.15-rc.1", LegacyTag, "v65536.0.0", "v1.65536.0", "v1.0.65536", "v1.0.18446744073709551616"} {
		if err := validateTagState(tag, "123", nil, nil); err == nil {
			t.Fatalf("invalid tag accepted: %s", tag)
		}
	}
	for _, tag := range []string{"v1.0.15", "v1.1.0", "v2.0.0"} {
		if err := validateTagState(tag, "123", []Ref{testRef("v1.0.14", testCommit), testRef(LegacyTag, testCommit)}, nil); err != nil {
			t.Fatal(err)
		}
	}
}
