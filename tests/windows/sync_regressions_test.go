package windows_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSyncSelectionErrorsFail(t *testing.T) {
	for _, tc := range []struct{ name, selection, message string }{
		{"regex", "  images-by-tag-regex:\n    team/image: '['\n", "invalid tag regex"},
		{"semver", "  images-by-semver:\n    team/image: 'definitely invalid'\n", "invalid semantic-version constraint"},
		{"repository", "  images:\n    UPPERCASE: [stable]\n", "invalid repository"},
		{"tag", "  images:\n    team/image: ['not a tag']\n", "invalid tag"},
		{"partial selection", "  images:\n    team/valid: [stable]\n  images-by-tag-regex:\n    team/image: '['\n", "invalid tag regex"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newCLI(t)
			config := filepath.Join(c.root, "sync.yaml")
			write(t, config, []byte("example.invalid:\n"+tc.selection))
			dest := filepath.Join(c.root, "must-not-exist")
			c.run(t, tc.message, "sync", "--src=yaml", "--dest=dir", config, dest)
			if _, err := os.Stat(dest); !os.IsNotExist(err) {
				t.Fatalf("invalid selection changed destination: %v", err)
			}
		})
	}
}

func TestSyncDiscoveryFailuresFail(t *testing.T) {
	for _, selection := range []string{
		"  images:\n    team/image: []\n",
		"  images-by-tag-regex:\n    team/image: '.*'\n",
		"  images-by-semver:\n    team/image: '>=1.0.0'\n",
	} {
		t.Run(strings.Fields(selection)[0], func(t *testing.T) {
			c := newCLI(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"errors":[{"code":"UNAUTHORIZED","message":"synthetic discovery rejection"}]}`))
			}))
			defer server.Close()
			host := strings.TrimPrefix(server.URL, "http://")
			config := filepath.Join(c.root, "sync.yaml")
			write(t, config, []byte(fmt.Sprintf("%s:\n  tls-verify: false\n%s", host, selection)))
			c.run(t, "synthetic discovery rejection", "sync", "--src=yaml", "--dest=docker", config, "example.invalid/mirror")
		})
	}
}

func TestSyncTagDiscoveryDeadline(t *testing.T) {
	for _, mode := range []string{"docker", "yaml-images", "yaml-regex", "yaml-semver"} {
		t.Run(mode, func(t *testing.T) {
			c := newCLI(t)
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Docker-Distribution-Api-Version", "registry/2.0")
				if strings.HasSuffix(r.URL.Path, "/tags/list") {
					requests.Add(1)
					select {
					case <-r.Context().Done():
						return
					case <-time.After(7 * time.Second):
					}
					_, _ = w.Write([]byte(`{"name":"team/image","tags":[]}`))
					return
				}
				_, _ = w.Write([]byte(`{}`))
			}))
			defer server.Close()
			host := strings.TrimPrefix(server.URL, "http://")
			args := []string{"--command-timeout=500ms", "sync", "--dest=docker"}
			if mode == "docker" {
				args = append(args, "--src=docker", "--src-tls-verify=false", host+"/team/image")
			} else {
				selection := "images:\n    team/image: []"
				if mode == "yaml-regex" {
					selection = "images-by-tag-regex:\n    team/image: '.*'"
				} else if mode == "yaml-semver" {
					selection = "images-by-semver:\n    team/image: '>=1.0.0'"
				}
				config := filepath.Join(c.root, "sync.yaml")
				write(t, config, []byte(fmt.Sprintf("%s:\n  tls-verify: false\n  %s\n", host, selection)))
				args = append(args, "--src=yaml", config)
			}
			args = append(args, "example.invalid/mirror")
			start := time.Now()
			c.run(t, "context deadline exceeded", args...)
			if elapsed := time.Since(start); elapsed > 3*time.Second || requests.Load() == 0 {
				t.Fatalf("tag-list deadline not enforced: elapsed=%s requests=%d", elapsed, requests.Load())
			}
		})
	}
}

func TestSyncDryRunDoesNotWrite(t *testing.T) {
	c := newCLI(t)
	dest := filepath.Join(c.root, "absent", "nested", "images")
	digests := filepath.Join(c.root, "absent", "digests.txt")
	args := []string{"sync", "--dry-run", "--src=docker", "--dest=dir", "--digestfile", digests, "example.invalid/team/image:Stable", dest}
	c.run(t, "", args...)
	if _, err := os.Stat(filepath.Join(c.root, "absent")); !os.IsNotExist(err) {
		t.Fatalf("dry run created destination or digest tree: %v", err)
	}
	args = append(args[:1], append([]string{"--append-suffix=/../invalid"}, args[1:]...)...)
	c.run(t, "invalid destination image", args...)
	if _, err := os.Stat(filepath.Join(c.root, "absent")); !os.IsNotExist(err) {
		t.Fatalf("invalid dry run created destination: %v", err)
	}
}

func TestSyncRejectsUnsafeEncodedDirectoryNames(t *testing.T) {
	for _, component := range []string{"~skopeo~%2e%2e", "~skopeo~%2fescape", "~skopeo~%5cescape", "~skopeo~%GG"} {
		t.Run(component, func(t *testing.T) {
			c, f := newCLI(t), makeFixture(t)
			source := filepath.Join(c.root, "input")
			copyFixtureDirectory(t, f.dir, filepath.Join(source, component))
			c.run(t, "invalid encoded sync directory", "sync", "--src=dir", "--dest=docker", source, "example.invalid/mirror")
		})
	}
}

func TestWindowsRegistryDirectoryRoundTrip(t *testing.T) {
	for _, scoped := range []bool{false, true} {
		t.Run(fmt.Sprintf("scoped=%v", scoped), func(t *testing.T) {
			c, f, host := newCLI(t), makeFixture(t), startRegistry(t)
			for _, tag := range []string{"stable", "Stable", "v1.2.3"} {
				c.run(t, "", "copy", "--all", "--dest-tls-verify=false", "dir:"+f.dir, "docker://"+host+"/team/image:"+tag)
			}
			dest := filepath.Join(c.root, "mirror")
			args := []string{"sync", "--all", "--src=docker", "--dest=dir", "--src-tls-verify=false"}
			if scoped {
				args = append(args, "--scoped")
			}
			c.run(t, "", append(args, host+"/team/image", dest)...)
			restore := dest
			wantRepo := "restored/image"
			if scoped {
				// The export's first directory is the encoded source authority.
				// Restore relative to that registry to preserve its namespace.
				entries, err := os.ReadDir(dest)
				if err != nil || len(entries) != 1 {
					t.Fatalf("unexpected source registry directories: %v %v", entries, err)
				}
				restore = filepath.Join(dest, entries[0].Name())
				wantRepo = "restored/team/image"
			}
			restoreArgs := []string{"sync", "--all", "--src=dir", "--dest=docker", "--dest-tls-verify=false"}
			if scoped {
				restoreArgs = append(restoreArgs, "--scoped")
			}
			c.run(t, "", append(restoreArgs, restore, host+"/restored")...)
			for _, tag := range []string{"stable", "Stable", "v1.2.3"} {
				src := c.run(t, "", "inspect", "--raw", "--tls-verify=false", "docker://"+host+"/team/image:"+tag)
				dst := c.run(t, "", "inspect", "--raw", "--tls-verify=false", "docker://"+host+"/"+wantRepo+":"+tag)
				if !bytes.Equal(src, dst) {
					t.Fatalf("round trip changed manifest for %s", tag)
				}
			}
			// Existing output must remain unchanged rather than be overwritten.
			c.run(t, "Refusing to overwrite", append(args, host+"/team/image:stable", dest)...)
		})
	}
}

func TestWindowsDigestAndReservedNamespaceRoundTrip(t *testing.T) {
	c, f, host := newCLI(t), makeFixture(t), startRegistry(t)
	ref := "docker://" + host + "/con/image:stable"
	c.run(t, "", "copy", "--all", "--dest-tls-verify=false", "dir:"+f.dir, ref)
	raw := c.run(t, "", "inspect", "--raw", "--tls-verify=false", ref)
	digest := hash(raw)
	dest := filepath.Join(c.root, "mirror")
	c.run(t, "", "sync", "--all", "--scoped", "--src=docker", "--dest=dir", "--src-tls-verify=false", host+"/con/image@"+digest, dest)
	entries, err := os.ReadDir(dest)
	if err != nil || len(entries) != 1 {
		t.Fatalf("unexpected exported registry directories: %v %v", entries, err)
	}
	root := filepath.Join(dest, entries[0].Name())
	c.run(t, "", "sync", "--all", "--scoped", "--src=dir", "--dest=docker", "--dest-tls-verify=false", root, host+"/restored")
	restored := c.run(t, "", "inspect", "--raw", "--tls-verify=false", "docker://"+host+"/restored/con/image@"+digest)
	if !bytes.Equal(raw, restored) {
		t.Fatal("digest or reserved namespace changed during directory round trip")
	}
}

func TestSyncValidEmptySelectionSucceeds(t *testing.T) {
	c := newCLI(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Docker-Distribution-Api-Version", "registry/2.0")
		if strings.HasSuffix(r.URL.Path, "/tags/list") {
			_, _ = w.Write([]byte(`{"name":"team/image","tags":["stable"]}`))
		} else {
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer server.Close()
	config := filepath.Join(c.root, "sync.yaml")
	write(t, config, []byte(fmt.Sprintf("%s:\n  tls-verify: false\n  images-by-tag-regex:\n    team/image: '^never-matches$'\n", strings.TrimPrefix(server.URL, "http://"))))
	c.run(t, "", "sync", "--src=yaml", "--dest=docker", config, "example.invalid/mirror")
}
