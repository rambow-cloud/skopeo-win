package windows_test

import (
	"debug/buildinfo"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/go-containerregistry/pkg/registry"
)

func TestRegistryAPIErrorRetries(t *testing.T) {
	for _, command := range []string{"inspect", "copy", "sync"} {
		for _, tc := range []struct {
			code     string
			status   int
			attempts int32
			exitCode int
		}{
			{"UNAUTHORIZED", http.StatusUnauthorized, 1, 1},
			{"DENIED", http.StatusForbidden, 1, 1},
			{"NAME_UNKNOWN", http.StatusNotFound, 1, 1},
			{"MANIFEST_UNKNOWN", http.StatusNotFound, 1, 2},
			{"UNKNOWN", http.StatusServiceUnavailable, 3, 1},
		} {
			t.Run(command+"/"+tc.code, func(t *testing.T) {
				c := newCLI(t)
				var attempts atomic.Int32
				message := "fixture-" + strings.ToLower(tc.code)
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Docker-Distribution-Api-Version", "registry/2.0")
					w.Header().Set("Content-Type", "application/json")
					if r.URL.Path == "/v2/" {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte("{}"))
						return
					}
					if strings.Contains(r.URL.Path, "/manifests/") {
						attempts.Add(1)
						w.WriteHeader(tc.status)
						_ = json.NewEncoder(w).Encode(map[string]any{"errors": []map[string]string{{"code": tc.code, "message": message}}})
						return
					}
					w.WriteHeader(http.StatusNotFound)
				}))
				defer server.Close()
				host := strings.TrimPrefix(server.URL, "http://")
				ref := host + "/team/image:stable"
				args := []string{command, "--retry-times=2", "--retry-delay=1ms"}
				switch command {
				case "inspect":
					args = append(args, "--tls-verify=false", "docker://"+ref)
				case "copy":
					args = append(args, "--src-tls-verify=false", "docker://"+ref, "dir:"+filepath.Join(c.root, "destination"))
				case "sync":
					args = append(args, "--src=docker", "--dest=dir", "--src-tls-verify=false", ref, filepath.Join(c.root, "mirror"))
				}
				want := message
				if tc.status == http.StatusServiceUnavailable {
					want = "503"
				}
				result := c.execute(t, want, nil, args...)
				if result.exitCode != tc.exitCode || attempts.Load() != tc.attempts {
					t.Fatalf("wrong registry error/retry behavior: code=%d attempts=%d want=%d stderr=%s", result.exitCode, attempts.Load(), tc.attempts, result.stderr)
				}
			})
		}
	}
}

func TestReleaseRegistryDependencyGraph(t *testing.T) {
	for _, arch := range []string{"amd64", "arm64"} {
		t.Run(arch, func(t *testing.T) {
			info, err := buildinfo.ReadFile(filepath.Join(filepath.Dir(executable), "skopeo-windows-"+arch+".exe"))
			if err != nil {
				t.Fatal(err)
			}
			foundCurrent := false
			foundForks := 0
			for _, module := range info.Deps {
				if module.Path == "github.com/docker/distribution" || module.Path == "github.com/distribution/distribution" {
					t.Fatalf("legacy registry module is still linked: %+v", module)
				}
				if module.Path == "github.com/distribution/distribution/v3" {
					var major, minor, patch int
					n, _ := fmt.Sscanf(module.Version, "v%d.%d.%d", &major, &minor, &patch)
					if n != 3 || major != 3 || strings.Contains(module.Version, "-") || (minor < 1 || minor == 1 && patch < 2) {
						t.Fatalf("Distribution security baseline is not met: %s", module.Version)
					}
					foundCurrent = true
				}
				if module.Path == "go.podman.io/common" || module.Path == "go.podman.io/image/v5" {
					if module.Replace == nil || !strings.HasPrefix(module.Replace.Path, "github.com/rambow-cloud/container-libs/") || module.Replace.Version == "" {
						t.Fatalf("registry library does not use the versioned reviewed sources: %+v", module)
					}
					foundForks++
				}
			}
			if !foundCurrent || foundForks != 2 {
				t.Fatalf("incomplete registry dependency migration: current=%v forks=%d", foundCurrent, foundForks)
			}
		})
	}
}

func TestRegistryMissingBlobError(t *testing.T) {
	c, f := newCLI(t), makeFixture(t)
	backend := registry.New(registry.Logger(log.New(io.Discard, "", 0)))
	var failBlob atomic.Bool
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if failBlob.Load() && r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/blobs/"+f.layerDigest) {
			requests.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"code":"BLOB_UNKNOWN","message":"fixture-layer-missing"}]}`))
			return
		}
		backend.ServeHTTP(w, r)
	}))
	defer server.Close()
	ref := "docker://" + strings.TrimPrefix(server.URL, "http://") + "/team/image:stable"
	c.run(t, "", "copy", "--all", "--dest-tls-verify=false", "dir:"+f.dir, ref)
	failBlob.Store(true)
	result := c.execute(t, "fixture-layer-missing", nil, "copy", "--retry-times=0", "--src-tls-verify=false", ref, "dir:"+filepath.Join(c.root, "missing-layer"))
	if result.exitCode != 1 || requests.Load() == 0 {
		t.Fatalf("missing blob did not fail through registry API: requests=%d stderr=%s", requests.Load(), result.stderr)
	}
}

func TestRegistryBlobMirrorFallback(t *testing.T) {
	c, f := newCLI(t), makeFixture(t)
	originBackend := registry.New(registry.Logger(log.New(io.Discard, "", 0)))
	mirrorBackend := registry.New(registry.Logger(log.New(io.Discard, "", 0)))
	var failMirror atomic.Bool
	var originReads, mirrorFailures atomic.Int32
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/blobs/"+f.layerDigest) {
			originReads.Add(1)
		}
		originBackend.ServeHTTP(w, r)
	}))
	defer origin.Close()
	mirror := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if failMirror.Load() && r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/blobs/"+f.layerDigest) {
			mirrorFailures.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"code":"BLOB_UNKNOWN","message":"fixture-mirror-layer-missing"}]}`))
			return
		}
		mirrorBackend.ServeHTTP(w, r)
	}))
	defer mirror.Close()
	originHost, mirrorHost := strings.TrimPrefix(origin.URL, "http://"), strings.TrimPrefix(mirror.URL, "http://")
	for _, host := range []string{originHost, mirrorHost} {
		c.run(t, "", "copy", "--all", "--dest-tls-verify=false", "dir:"+f.dir, "docker://"+host+"/team/image:stable")
	}
	config := fmt.Sprintf("unqualified-search-registries = []\n[[registry]]\nprefix = \"registry.example.test/team\"\nlocation = %q\ninsecure = true\n[[registry.mirror]]\nlocation = %q\ninsecure = true\n", originHost+"/team", mirrorHost+"/team")
	write(t, filepath.Join(c.root, "registries.conf"), []byte(config))
	failMirror.Store(true)
	dest := filepath.Join(c.root, "recovered")
	c.run(t, "", "copy", "--retry-times=0", "--src-tls-verify=false", "docker://registry.example.test/team/image:stable", "dir:"+dest)
	if mirrorFailures.Load() == 0 || originReads.Load() == 0 {
		t.Fatalf("blob fallback was not exercised: mirror=%d origin=%d", mirrorFailures.Load(), originReads.Load())
	}
	if got := hash(read(t, filepath.Join(dest, encoded(f.layerDigest)))); got != f.layerDigest {
		t.Fatalf("fallback layer digest mismatch: %s", got)
	}
}

