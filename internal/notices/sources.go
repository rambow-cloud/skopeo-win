package notices

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// ReviewedSource binds a versioned remote fork to reviewed, byte-exact vendor sources.
type ReviewedSource struct {
	Module             string `json:"module"`
	Version            string `json:"version"`
	Replacement        Module `json:"replacement"`
	Repository         string `json:"repository"`
	Commit             string `json:"commit"`
	Directory          string `json:"directory"`
	UpstreamRepository string `json:"upstreamRepository"`
	UpstreamCommit     string `json:"upstreamCommit"`
	VendorSHA256       string `json:"vendorSHA256"`
}

var sourceCommitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
var sourceDigestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
var sourceVersionPattern = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?(?:\+incompatible)?$`)
var sourceRepositoryPattern = regexp.MustCompile(`^https://github\.com/[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

func loadReviewedSources(repository string) (map[string]ReviewedSource, error) {
	data, err := os.ReadFile(filepath.Join(repository, "packaging", "store", "module-sources.json"))
	if os.IsNotExist(err) {
		return map[string]ReviewedSource{}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(data) > 1<<20 {
		return nil, fmt.Errorf("reviewed source policy is oversized")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var entries []ReviewedSource
	if err := decoder.Decode(&entries); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("trailing reviewed source policy content")
	}
	sources := map[string]ReviewedSource{}
	for _, entry := range entries {
		remote := strings.TrimPrefix(entry.Repository, "https://") + "/" + entry.Directory
		versionedRemote := strings.TrimSuffix(entry.Replacement.Path, "/v"+strings.Split(strings.TrimPrefix(entry.Version, "v"), ".")[0])
		if !fs.ValidPath(entry.Module) || strings.ContainsAny(entry.Module, ":\\\r\n") ||
			!fs.ValidPath(entry.Directory) || entry.Directory == "." || strings.ContainsAny(entry.Directory, ":\\\r\n") ||
			!sourceRepositoryPattern.MatchString(entry.Repository) || !sourceRepositoryPattern.MatchString(entry.UpstreamRepository) ||
			!sourceCommitPattern.MatchString(entry.Commit) || !sourceCommitPattern.MatchString(entry.UpstreamCommit) ||
			!sourceVersionPattern.MatchString(entry.Version) || !sourceVersionPattern.MatchString(entry.Replacement.Version) ||
			!sourceDigestPattern.MatchString(entry.VendorSHA256) || entry.Replacement.Replacement != nil ||
			(entry.Replacement.Path != remote && versionedRemote != remote) {
			return nil, fmt.Errorf("invalid reviewed source record for %s", entry.Module)
		}
		if _, exists := sources[entry.Module]; exists {
			return nil, fmt.Errorf("duplicate reviewed source %s", entry.Module)
		}
		sources[entry.Module] = entry
	}
	return sources, nil
}

// SourceDigest rejects links and hashes sorted relative paths and exact file bytes.
func SourceDigest(root string) (string, error) {
	info, err := os.Lstat(root)
	if err != nil {
		return "", err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("source root must be a real directory")
	}
	files := map[string]string{}
	err = filepath.WalkDir(root, func(file string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("source contains a non-regular file")
		}
		relative, err := filepath.Rel(root, file)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(relative)] = digest(data)
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", fmt.Errorf("source directory is empty")
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	slices.Sort(names)
	hash := sha256.New()
	for _, name := range names {
		fmt.Fprintf(hash, "%s\x00%s\n", name, files[name])
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
