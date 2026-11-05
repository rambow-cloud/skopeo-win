package windows_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/registry"
)

func setCLIEnv(c *cli, key, value string) {
	for i, item := range c.env {
		name, _, _ := strings.Cut(item, "=")
		if strings.EqualFold(name, key) {
			c.env[i] = key + "=" + value
			return
		}
	}
	c.env = append(c.env, key+"="+value)
}

func publicMirror(t *testing.T, c cli, namespace, host string, insecure bool) string {
	t.Helper()
	write(t, filepath.Join(c.root, "registries.conf"), []byte(fmt.Sprintf("[[registry]]\nprefix = %q\nlocation = %q\ninsecure = %t\n", namespace, host+"/library", insecure)))
	return "docker://" + namespace + "/demo:stable"
}

func publicFixture(t *testing.T) (cli, string, string) {
	t.Helper()
	c, host := newCLI(t), startRegistry(t)
	f := makePlatformFixture(t, "linux", "windows")
	c.run(t, "", "copy", "--all", "--dest-tls-verify=false", "dir:"+f.dir, "docker://"+host+"/library/demo:stable")
	c.defaultPolicy = true
	c.imageOS = ""
	return c, publicMirror(t, c, "docker.io/library", host, true), host
}

func assertNoDefaultPolicy(t *testing.T, root string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, "containers", "policy.json")); !os.IsNotExist(err) {
		t.Fatalf("read-only or explicitly configured command wrote a default policy: %v", err)
	}
}

func assertImagePlatform(t *testing.T, c cli, ref, system, arch string) {
	t.Helper()
	var config struct{ OS, Architecture string }
	if err := json.Unmarshal(c.run(t, "", "inspect", "--config", ref), &config); err != nil {
		t.Fatal(err)
	}
	if config.OS != system || config.Architecture != arch {
		t.Fatalf("selected %s/%s, expected %s/%s", config.OS, config.Architecture, system, arch)
	}
}

func TestFirstRunDefaultPolicyAndLinuxPlatform(t *testing.T) {
	c, ref, _ := publicFixture(t)
	assertImagePlatform(t, c, ref, "linux", runtime.GOARCH)
	c.run(t, "", "--version")
	c.run(t, "", "--help")
	assertNoDefaultPolicy(t, c.root)
	c.run(t, "", "copy", ref, "docker-archive:./default-image.tar:demo:stable")
	assertImagePlatform(t, c, "docker-archive:./default-image.tar", "linux", runtime.GOARCH)
	policyPath := filepath.Join(c.root, "containers", "policy.json")
	var policy map[string]any
	if err := json.Unmarshal(read(t, policyPath), &policy); err != nil {
		t.Fatalf("generated policy is incomplete: %v", err)
	}
	for _, tc := range []struct{ system, arch string }{{"windows", "amd64"}, {"windows", "arm64"}, {"linux", "arm64"}} {
		t.Run(tc.system+"/"+tc.arch, func(t *testing.T) {
			dest := "dir:" + filepath.Join(c.root, tc.system+"-"+tc.arch)
			c.run(t, "", "--override-os", tc.system, "--override-arch", tc.arch, "copy", ref, dest)
			assertImagePlatform(t, c, dest, tc.system, tc.arch)
		})
	}
	allPlatforms := "oci:" + filepath.Join(c.root, "all-platforms") + ":stable"
	c.run(t, "", "copy", "--all", ref, allPlatforms)
	getIndex(t, c.run(t, "", "inspect", "--raw", allPlatforms), 4)
}

func TestDefaultPolicyNamespaceBoundaries(t *testing.T) {
	for _, tc := range []struct {
		namespace string
		allowed   bool
	}{
		{"docker.io/library", true}, {"mcr.microsoft.com", true}, {"registry.k8s.io", true},
		{"quay.io/prometheus", true}, {"ghcr.io/fluxcd", true},
		{"docker.io/community", false}, {"docker.io/library-evil", false},
		{"mcr.microsoft.com.evil.invalid", false}, {"registry.k8s.io.evil.invalid", false},
		{"quay.io/prometheus-evil", false}, {"ghcr.io/community", false},
	} {
		t.Run(tc.namespace, func(t *testing.T) {
			c, _, host := publicFixture(t)
			ref := publicMirror(t, c, tc.namespace, host, true)
			want := "rejected"
			if tc.allowed {
				want = ""
			}
			c.run(t, want, "copy", ref, "dir:"+filepath.Join(c.root, "copy"))
		})
	}
	t.Run("require-signed still rejects unsigned public images", func(t *testing.T) {
		c, ref, _ := publicFixture(t)
		c.run(t, "signature", "--require-signed", "copy", ref, "dir:"+filepath.Join(c.root, "signed-only"))
	})
	t.Run("allowlisted namespaces still require valid TLS", func(t *testing.T) {
		c := newCLI(t)
		s := httptest.NewTLSServer(registry.New(registry.Logger(log.New(io.Discard, "", 0))))
		t.Cleanup(s.Close)
		ref := publicMirror(t, c, "docker.io/library", strings.TrimPrefix(s.URL, "https://"), false)
		c.defaultPolicy = true
		c.run(t, "certificate", "copy", ref, "dir:"+filepath.Join(c.root, "tls-rejected"))
	})
}

func TestDefaultPolicyPreservesConfiguredPolicies(t *testing.T) {
	for _, tc := range []struct{ name, data, want string }{
		{"rejecting user policy", `{"default":[{"type":"reject"}]}`, "rejected"},
		{"malformed user policy", `{"default":`, "invalid policy"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, ref, _ := publicFixture(t)
			path := filepath.Join(c.root, "containers", "policy.json")
			write(t, path, []byte(tc.data))
			c.run(t, tc.want, "copy", ref, "dir:"+filepath.Join(c.root, "copy"))
			if !bytes.Equal(read(t, path), []byte(tc.data)) {
				t.Fatal("existing policy was replaced")
			}
		})
	}
	for _, kind := range []string{"system", "explicit", "environment"} {
		t.Run(kind, func(t *testing.T) {
			c, ref, _ := publicFixture(t)
			path := filepath.Join(c.root, "system", "containers", "policy.json")
			if kind != "system" {
				path = filepath.Join(c.root, "custom-policy.json")
			}
			write(t, path, []byte(`{"default":[{"type":"reject"}]}`))
			if kind == "explicit" {
				c.policy = path
			}
			if kind == "environment" {
				setCLIEnv(&c, "CONTAINERS_POLICY_JSON", path)
			}
			c.run(t, "rejected", "copy", ref, "dir:"+filepath.Join(c.root, "copy"))
			assertNoDefaultPolicy(t, c.root)
		})
	}
	for _, kind := range []string{"explicit", "environment"} {
		t.Run("missing "+kind+" policy is an error", func(t *testing.T) {
			c, ref, _ := publicFixture(t)
			path := filepath.Join(c.root, "missing-policy.json")
			if kind == "explicit" {
				c.policy = path
			} else {
				setCLIEnv(&c, "CONTAINERS_POLICY_JSON", path)
			}
			c.run(t, "missing-policy.json", "copy", ref, "dir:"+filepath.Join(c.root, "copy"))
			assertNoDefaultPolicy(t, c.root)
		})
	}
}

func TestEmbeddedPolicyWhenAppDataIsUnavailable(t *testing.T) {
	c, ref, host := publicFixture(t)
	appData := filepath.Join(c.root, "unavailable", "roaming")
	setCLIEnv(&c, "APPDATA", appData)
	c.run(t, "", "copy", ref, "dir:"+filepath.Join(c.root, "copy"))
	assertNoDefaultPolicy(t, appData)
	ref = publicMirror(t, c, "docker.io/community", host, true)
	c.run(t, "rejected", "copy", ref, "dir:"+filepath.Join(c.root, "blocked"))
}

func TestDefaultPolicyDryRunDoesNotWrite(t *testing.T) {
	c, ref, _ := publicFixture(t)
	dest := filepath.Join(c.root, "absent", "images")
	c.run(t, "", "sync", "--dry-run", "--src=docker", "--dest=dir", strings.TrimPrefix(ref, "docker://"), dest)
	assertNoDefaultPolicy(t, c.root)
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("dry run created output: %v", err)
	}
}

func TestConcurrentFirstRunPolicyCreation(t *testing.T) {
	c, ref, _ := publicFixture(t)
	t.Run("concurrent writers", func(t *testing.T) {
		for i := range 8 {
			t.Run(fmt.Sprint(i), func(t *testing.T) {
				t.Parallel()
				c.run(t, "", "copy", ref, "dir:"+filepath.Join(c.root, fmt.Sprintf("copy-%d", i)))
			})
		}
	})
	var policy map[string]any
	if err := json.Unmarshal(read(t, filepath.Join(c.root, "containers", "policy.json")), &policy); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(c.root, "containers"))
	if err != nil || len(entries) != 1 || entries[0].Name() != "policy.json" {
		t.Fatalf("bootstrap left temporary files: %v %v", entries, err)
	}
}
