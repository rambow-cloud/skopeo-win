// Package release implements independent version allocation and immutable publication.
package release

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const LegacyTag = "v1.23.0"

var stable = regexp.MustCompile(`^v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$`)

type Ref struct {
	Ref    string `json:"ref"`
	Object struct {
		Type string `json:"type"`
		SHA  string `json:"sha"`
	} `json:"object"`
}

type Asset struct {
	Name   string `json:"name"`
	Digest string `json:"digest"`
}

type Release struct {
	Tag    string  `json:"tag_name"`
	Body   string  `json:"body"`
	Draft  bool    `json:"draft"`
	Assets []Asset `json:"assets"`
}

func marker(runID string) string { return "<!-- skopeo-win-release-run:" + runID + " -->" }

func parse(tag string) ([3]uint64, bool) {
	var v [3]uint64
	parts := stable.FindStringSubmatch(tag)
	if parts == nil {
		return v, false
	}
	for i := range v {
		n, err := strconv.ParseUint(parts[i+1], 10, 64)
		if err != nil {
			return v, false
		}
		v[i] = n
	}
	return v, true
}

// Next excludes the historical upstream-aligned tag and reserves draft/orphan tags.
// A release already associated with this workflow run is reused for safe retries.
func Next(refs []Ref, releases []Release, runID, commit string) (string, error) {
	for _, r := range releases {
		if !strings.Contains(r.Body, marker(runID)) {
			continue
		}
		if _, ok := parse(r.Tag); !ok || r.Tag == LegacyTag {
			return "", fmt.Errorf("invalid retry release tag %q", r.Tag)
		}
		if err := CheckTag(refs, r.Tag, commit, true); err != nil {
			return "", err
		}
		return strings.TrimPrefix(r.Tag, "v"), nil
	}
	var highest [3]uint64
	found := false
	tags := make([]string, 0, len(refs)+len(releases))
	for _, ref := range refs {
		tags = append(tags, strings.TrimPrefix(ref.Ref, "refs/tags/"))
	}
	for _, r := range releases {
		tags = append(tags, r.Tag)
	}
	for _, tag := range tags {
		v, ok := parse(tag)
		if !ok || tag == LegacyTag {
			continue
		}
		if !found || v[0] > highest[0] || v[0] == highest[0] && (v[1] > highest[1] || v[1] == highest[1] && v[2] > highest[2]) {
			highest, found = v, true
		}
	}
	if !found {
		return "1.0.0", nil
	}
	if highest[2] == ^uint64(0) {
		return "", fmt.Errorf("release patch version overflow")
	}
	highest[2]++
	tag := fmt.Sprintf("v%d.%d.%d", highest[0], highest[1], highest[2])
	if tag == LegacyTag {
		tag = "v1.23.1"
	}
	return strings.TrimPrefix(tag, "v"), nil
}

func CheckTag(refs []Ref, tag, commit string, required bool) error {
	for _, ref := range refs {
		if ref.Ref != "refs/tags/"+tag {
			continue
		}
		if ref.Object.Type != "commit" || ref.Object.SHA != commit {
			return fmt.Errorf("refusing to move existing tag %s", tag)
		}
		return nil
	}
	if required {
		return fmt.Errorf("release tag %s is missing", tag)
	}
	return nil
}

type Runner func(args ...string) ([]byte, error)

func GH(args ...string) ([]byte, error) {
	cmd := exec.Command("gh", args...)
	cmd.Stderr = os.Stderr
	return cmd.Output()
}

type Manager struct {
	Run                 Runner
	Repo, Commit, RunID string
}

func (m Manager) State() ([]Ref, []Release, error) {
	var refs []Ref
	var releases []Release
	for _, item := range []struct {
		path string
		refs bool
	}{
		{"git/matching-refs/tags/v", true}, {"releases", false},
	} {
		data, err := m.Run("api", "repos/"+m.Repo+"/"+item.path, "--paginate", "--slurp")
		if err != nil {
			return nil, nil, err
		}
		if item.refs {
			var pages [][]Ref
			if err := json.Unmarshal(data, &pages); err != nil {
				return nil, nil, err
			}
			for _, page := range pages {
				refs = append(refs, page...)
			}
		} else {
			var pages [][]Release
			if err := json.Unmarshal(data, &pages); err != nil {
				return nil, nil, err
			}
			for _, page := range pages {
				releases = append(releases, page...)
			}
		}
	}
	return refs, releases, nil
}

// Files requires exactly the expected files and verifies checksums before mutation.
func Files(dir string) (map[string]string, error) {
	files := map[string]string{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, arch := range []string{"amd64", "arm64"} {
		name := "skopeo-windows-" + arch + ".exe"
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(data)
		checksum, err := os.ReadFile(filepath.Join(dir, name+".sha256"))
		if err != nil {
			return nil, err
		}
		fields := strings.Fields(string(checksum))
		if len(fields) != 2 || fields[0] != hex.EncodeToString(sum[:]) || fields[1] != name {
			return nil, fmt.Errorf("invalid checksum file for %s", name)
		}
		files[name] = "sha256:" + hex.EncodeToString(sum[:])
		sum = sha256.Sum256(checksum)
		files[name+".sha256"] = "sha256:" + hex.EncodeToString(sum[:])
	}
	if len(entries) != len(files) {
		return nil, fmt.Errorf("unexpected release files")
	}
	return files, nil
}

func ValidateAssets(assets []Asset, expected map[string]string, complete bool) error {
	seen := map[string]bool{}
	for _, asset := range assets {
		if seen[asset.Name] || expected[asset.Name] == "" || asset.Digest != expected[asset.Name] {
			return fmt.Errorf("refusing to replace mismatched release asset %s", asset.Name)
		}
		seen[asset.Name] = true
	}
	if complete && len(seen) != len(expected) {
		return fmt.Errorf("published release has missing assets")
	}
	return nil
}

func (m Manager) Publish(version, dir string) error {
	tag := "v" + version
	if _, ok := parse(tag); !ok || tag == LegacyTag {
		return fmt.Errorf("invalid independent release version %q", version)
	}
	expected, err := Files(dir)
	if err != nil {
		return err
	}
	refs, releases, err := m.State()
	if err != nil {
		return err
	}
	if err := CheckTag(refs, tag, m.Commit, false); err != nil {
		return err
	}
	var current *Release
	for i := range releases {
		if releases[i].Tag == tag {
			current = &releases[i]
			break
		}
	}
	if current != nil {
		if !strings.Contains(current.Body, marker(m.RunID)) {
			return fmt.Errorf("release %s belongs to a different run", tag)
		}
		if err := CheckTag(refs, tag, m.Commit, true); err != nil {
			return err
		}
		if err := ValidateAssets(current.Assets, expected, !current.Draft); err != nil {
			return err
		}
		if !current.Draft {
			return nil
		} // Published releases are immutable, including on reruns.
	} else {
		hasTag := false
		for _, ref := range refs {
			if ref.Ref == "refs/tags/"+tag {
				hasTag = true
			}
		}
		if !hasTag {
			// Reserve a lightweight tag atomically; never PATCH or force an existing ref.
			if _, err := m.Run("api", "--method", "POST", "repos/"+m.Repo+"/git/refs", "-f", "ref=refs/tags/"+tag, "-f", "sha="+m.Commit); err != nil {
				return err
			}
		}
		notes := fmt.Sprintf("%s\n\nIndependent skopeo-win %s.\n\nSource: https://github.com/%s/commit/%s\nBuild and security scans: https://github.com/%s/actions/runs/%s\n\nNative Windows amd64 and arm64 binaries with SHA-256 checksums. CGO is disabled. Executables are not Authenticode signed.", marker(m.RunID), version, m.Repo, m.Commit, m.Repo, m.RunID)
		_, err = m.Run("release", "create", tag, "--repo", m.Repo, "--target", m.Commit, "--title", "skopeo-win "+tag, "--notes", notes, "--draft", "--verify-tag")
		if err != nil {
			return err
		}
		current = &Release{Tag: tag, Draft: true}
	}
	for _, name := range []string{"skopeo-windows-amd64.exe", "skopeo-windows-amd64.exe.sha256", "skopeo-windows-arm64.exe", "skopeo-windows-arm64.exe.sha256"} {
		found := false
		for _, asset := range current.Assets {
			if asset.Name == name {
				found = true
			}
		}
		if found {
			continue
		}
		if _, err := m.Run("release", "upload", tag, filepath.Join(dir, name), "--repo", m.Repo); err != nil {
			return err
		}
	}
	refs, releases, err = m.State()
	if err != nil {
		return err
	}
	if err := CheckTag(refs, tag, m.Commit, true); err != nil {
		return err
	}
	for _, r := range releases {
		if r.Tag != tag {
			continue
		}
		if err := ValidateAssets(r.Assets, expected, true); err != nil {
			return err
		}
		_, err = m.Run("release", "edit", tag, "--repo", m.Repo, "--draft=false", "--latest")
		return err
	}
	return fmt.Errorf("draft release %s disappeared", tag)
}
