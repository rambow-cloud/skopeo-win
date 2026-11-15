package windows_test

import (
	"debug/buildinfo"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRegistryAPIErrorRetries(t *testing.T) {
	for _, command := range []string{"inspect", "copy", "sync"} {
		for _, tc := range []struct {
			code     string
			status   int
			attempts int32
		}{
			{"UNAUTHORIZED", http.StatusUnauthorized, 1},
			{"DENIED", http.StatusForbidden, 1},
			{"NAME_UNKNOWN", http.StatusNotFound, 1},
			{"MANIFEST_UNKNOWN", http.StatusNotFound, 1},
			{"UNKNOWN", http.StatusServiceUnavailable, 3},
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
				result := c.execute(t, message, nil, args...)
				if result.exitCode != 1 || attempts.Load() != tc.attempts {
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
