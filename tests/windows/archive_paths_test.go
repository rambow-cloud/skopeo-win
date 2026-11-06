package windows_test

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func windowsImagePaths(name string) []struct {
	name string
	path func(string) string
} {
	absolute := func(root string) string { return filepath.Join(root, "image files", name) }
	return []struct {
		name string
		path func(string) string
	}{
		{"drive backslashes", absolute},
		{"drive forward slashes", func(root string) string { return filepath.ToSlash(absolute(root)) }},
		{"relative with spaces", func(string) string { return filepath.Join("image files", name) }},
		{"single letter relative", func(string) string { return "x" }},
		{"extended drive", func(root string) string { return `\\?\` + absolute(root) }},
	}
}

func prepareImagePath(t *testing.T, c cli, path string) {
	t.Helper()
	if !filepath.IsAbs(path) {
		path = filepath.Join(c.root, path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
}

func uncompressedArchiveLayer(t *testing.T, b []byte) []byte {
	t.Helper()
	if !bytes.HasPrefix(b, []byte{0x1f, 0x8b}) {
		return b
	}
	r, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	result, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func assertArchiveRestored(t *testing.T, c cli, source, directory string, f fixture) {
	t.Helper()
	destination := "dir:" + directory
	c.run(t, "", "copy", source, destination)
	assertImagePlatform(t, c, destination, "windows", "amd64")
	var m manifest
	if err := json.Unmarshal(c.run(t, "", "inspect", "--raw", destination), &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Layers) != 1 {
		t.Fatalf("expected one restored layer, got %d", len(m.Layers))
	}
	layer := read(t, filepath.Join(directory, encoded(m.Layers[0].Digest)))
	if !bytes.Equal(uncompressedArchiveLayer(t, layer), uncompressedArchiveLayer(t, f.layer)) {
		t.Fatal("archive round trip changed layer contents")
	}
}

func TestWindowsDockerArchivePaths(t *testing.T) {
	for _, tc := range windowsImagePaths("image.tar") {
		t.Run(tc.name, func(t *testing.T) {
			c, f := newCLI(t), makeFixture(t)
			c.defaultPolicy = true
			path := tc.path(c.root)
			prepareImagePath(t, c, path)
			archive := "docker-archive:" + path
			tag := "registry.example.com:5000/team/demo:stable"
			extra := "registry.example.com:5000/team/demo:extra"
			c.run(t, "", "copy", "--additional-tag", extra, "dir:"+singleImageFixture(t, f), archive+":"+tag)
			var listing struct{ Tags []string }
			if err := json.Unmarshal(c.run(t, "", "list-tags", archive), &listing); err != nil {
				t.Fatal(err)
			}
			if len(listing.Tags) != 2 || !slices.Contains(listing.Tags, tag) || !slices.Contains(listing.Tags, extra) {
				t.Fatalf("archive tags changed: %v", listing.Tags)
			}
			for j, selector := range []string{"", ":" + tag, ":@0"} {
				assertArchiveRestored(t, c, archive+selector, filepath.Join(c.root, fmt.Sprintf("restored-%d", j)), f)
			}
			c.run(t, "index", "copy", archive+":@-1", "dir:"+filepath.Join(c.root, "invalid-index"))
		})
	}
}

func TestWindowsOCIImagePaths(t *testing.T) {
	for _, transport := range []string{"oci", "oci-archive"} {
		t.Run(transport, func(t *testing.T) {
			for _, tc := range windowsImagePaths("layout") {
				t.Run(tc.name, func(t *testing.T) {
					c, f := newCLI(t), makeFixture(t)
					c.defaultPolicy = true
					path := tc.path(c.root)
					prepareImagePath(t, c, path)
					ref := transport + ":" + path
					image := "release:stable"
					c.run(t, "", "copy", "--format", "oci", "dir:"+singleImageFixture(t, f), ref+":"+image)
					assertArchiveRestored(t, c, ref+":"+image, filepath.Join(c.root, "restored-named"), f)
					assertArchiveRestored(t, c, ref, filepath.Join(c.root, "restored-default"), f)
				})
			}
		})
	}
}
