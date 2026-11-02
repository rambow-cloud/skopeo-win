package windows_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/registry"
)

const (
	imageType   = "application/vnd.oci.image.manifest.v1+json"
	indexType   = "application/vnd.oci.image.index.v1+json"
	configType  = "application/vnd.oci.image.config.v1+json"
	layerType   = "application/vnd.oci.image.layer.v1.tar+gzip"
	foreignType = "application/vnd.oci.image.layer.nondistributable.v1.tar+gzip"
)

var executable string

func TestMain(m *testing.M) {
	var err error
	executable, err = filepath.Abs(os.Getenv("SKOPEO_BINARY"))
	if os.Getenv("SKOPEO_BINARY") == "" || err != nil {
		fmt.Fprintln(os.Stderr, "SKOPEO_BINARY must identify the built Windows executable")
		os.Exit(2)
	}
	if info, err := os.Stat(executable); err != nil || info.IsDir() {
		fmt.Fprintln(os.Stderr, "built executable is missing:", executable)
		os.Exit(2)
	}
	os.Exit(m.Run())
}

type cli struct {
	root   string
	env    []string
	policy string
}

func newCLI(t *testing.T) cli {
	t.Helper()
	root := t.TempDir()
	write(t, filepath.Join(root, "registries.conf"), []byte("unqualified-search-registries = []\n"))
	write(t, filepath.Join(root, "auth.json"), []byte(`{"auths":{}}`))
	if err := os.MkdirAll(filepath.Join(root, "registries.d"), 0o755); err != nil {
		t.Fatal(err)
	}
	env := []string{}
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		switch strings.ToUpper(key) {
		case "HOME", "USERPROFILE", "APPDATA", "LOCALAPPDATA", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_RUNTIME_DIR", "REGISTRY_AUTH_FILE", "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY":
			continue
		}
		env = append(env, item)
	}
	for _, key := range []string{"HOME", "USERPROFILE", "APPDATA", "LOCALAPPDATA", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_RUNTIME_DIR"} {
		env = append(env, key+"="+root)
	}
	env = append(env, "REGISTRY_AUTH_FILE="+filepath.Join(root, "auth.json"), "NO_PROXY=127.0.0.1,localhost,::1")
	return cli{root: root, env: env}
}

func (c cli) run(t *testing.T, wantError string, args ...string) []byte {
	t.Helper()
	return c.execute(t, wantError, nil, args...).stdout
}

type commandResult struct {
	stdout, stderr []byte
	exitCode       int
}

func (c cli) execute(t *testing.T, wantError string, afterStart func(context.Context), args ...string) commandResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	global := []string{"--registries-conf", filepath.Join(c.root, "registries.conf"), "--registries.d", filepath.Join(c.root, "registries.d")}
	if c.policy == "" {
		global = append(global, "--insecure-policy")
	} else {
		global = append(global, "--policy", c.policy)
	}
	cmd := exec.CommandContext(ctx, executable, append(global, args...)...)
	cmd.Dir, cmd.Env = c.root, c.env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Start()
	waited := false
	defer func() {
		if err == nil && !waited {
			cancel()
			_ = cmd.Wait()
		}
	}()
	if err == nil {
		if afterStart != nil {
			afterStart(ctx)
		}
		err = cmd.Wait()
		waited = true
	}
	t.Logf("skopeo %s\nexit=%v\nstdout=%s\nstderr=%s", strings.Join(args, " "), err, stdout.String(), stderr.String())
	if ctx.Err() != nil {
		t.Fatalf("command timed out: %v", ctx.Err())
	}
	if wantError == "" {
		if err != nil {
			t.Fatalf("command failed: %v\n%s", err, stderr.String())
		}
	} else {
		if _, ok := err.(*exec.ExitError); !ok || !strings.Contains(strings.ToLower(stderr.String()), strings.ToLower(wantError)) {
			t.Fatalf("expected nonzero exit containing %q, got %v: %s", wantError, err, stderr.String())
		}
	}
	code := 0
	if cmd.ProcessState != nil {
		code = cmd.ProcessState.ExitCode()
	}
	return commandResult{stdout.Bytes(), stderr.Bytes(), code}
}

func write(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func encode(t *testing.T, value any) []byte {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func hash(b []byte) string    { return fmt.Sprintf("sha256:%x", sha256.Sum256(b)) }
func encoded(d string) string { return strings.TrimPrefix(d, "sha256:") }

type platform struct {
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
}
type descriptor struct {
	MediaType string    `json:"mediaType"`
	Digest    string    `json:"digest"`
	Size      int       `json:"size"`
	Platform  *platform `json:"platform,omitempty"`
	URLs      []string  `json:"urls,omitempty"`
}
type manifest struct {
	SchemaVersion int          `json:"schemaVersion"`
	MediaType     string       `json:"mediaType"`
	Config        descriptor   `json:"config"`
	Layers        []descriptor `json:"layers"`
}
type index struct {
	SchemaVersion int          `json:"schemaVersion"`
	MediaType     string       `json:"mediaType"`
	Manifests     []descriptor `json:"manifests"`
}

type fixture struct {
	dir         string
	instances   []descriptor
	layer       []byte
	layerDigest string
	diffID      string
}

func makeFixture(t *testing.T) fixture {
	t.Helper()
	var raw, compressed bytes.Buffer
	tw := tar.NewWriter(&raw)
	payload := []byte("skopeo-win integration fixture\n")
	if err := tw.WriteHeader(&tar.Header{Name: "fixture.txt", Mode: 0o644, Size: int64(len(payload))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	zw := gzip.NewWriter(&compressed)
	if _, err := zw.Write(raw.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f := fixture{dir: t.TempDir(), layer: compressed.Bytes(), diffID: hash(raw.Bytes())}
	f.layerDigest = hash(f.layer)
	write(t, filepath.Join(f.dir, encoded(f.layerDigest)), f.layer)
	for _, arch := range []string{"amd64", "arm64"} {
		config := encode(t, map[string]any{"os": "windows", "architecture": arch, "rootfs": map[string]any{"type": "layers", "diff_ids": []string{f.diffID}}})
		cd := descriptor{MediaType: configType, Digest: hash(config), Size: len(config)}
		write(t, filepath.Join(f.dir, encoded(cd.Digest)), config)
		m := encode(t, manifest{2, imageType, cd, []descriptor{{MediaType: layerType, Digest: f.layerDigest, Size: len(f.layer)}}})
		d := descriptor{MediaType: imageType, Digest: hash(m), Size: len(m), Platform: &platform{"windows", arch}}
		write(t, filepath.Join(f.dir, encoded(d.Digest)+".manifest.json"), m)
		f.instances = append(f.instances, d)
	}
	write(t, filepath.Join(f.dir, "manifest.json"), encode(t, index{2, indexType, f.instances}))
	return f
}

func startRegistry(t *testing.T) string {
	t.Helper()
	s := httptest.NewServer(registry.New(registry.Logger(log.New(io.Discard, "", 0))))
	t.Cleanup(s.Close)
	return strings.TrimPrefix(s.URL, "http://")
}

func getIndex(t *testing.T, b []byte, count int) index {
	t.Helper()
	var i index
	if err := json.Unmarshal(b, &i); err != nil {
		t.Fatal(err)
	}
	if len(i.Manifests) != count {
		t.Fatalf("got %d platforms, want %d: %s", len(i.Manifests), count, b)
	}
	return i
}

func TestExecutableVersion(t *testing.T) {
	c := newCLI(t)
	output := string(c.run(t, "", "--version"))
	if expected := os.Getenv("SKOPEO_EXPECTED_VERSION"); expected != "" {
		if strings.TrimSpace(output) != "skopeo version "+expected {
			t.Fatalf("unexpected version: %s", output)
		}
	} else if !strings.HasPrefix(output, "skopeo version ") {
		t.Fatalf("unexpected version: %s", output)
	}
}

func TestRegistryRoundTrip(t *testing.T) {
	c, f, host := newCLI(t), makeFixture(t), startRegistry(t)
	ref := "docker://" + host + "/team/source:stable"
	c.run(t, "", "copy", "--all", "--dest-tls-verify=false", "dir:"+f.dir, ref)
	getIndex(t, c.run(t, "", "inspect", "--raw", "--tls-verify=false", ref), 2)
	var config platform
	if err := json.Unmarshal(c.run(t, "", "--override-os=windows", "--override-arch=arm64", "inspect", "--config", "--tls-verify=false", ref), &config); err != nil {
		t.Fatal(err)
	}
	if config.OS != "windows" || config.Architecture != "arm64" {
		t.Fatalf("wrong platform: %+v", config)
	}
	var tags struct {
		Tags []string `json:"Tags"`
	}
	if err := json.Unmarshal(c.run(t, "", "list-tags", "--tls-verify=false", "docker://"+host+"/team/source"), &tags); err != nil {
		t.Fatal(err)
	}
	if len(tags.Tags) != 1 || tags.Tags[0] != "stable" {
		t.Fatalf("wrong tags: %v", tags.Tags)
	}
	dest := filepath.Join(t.TempDir(), "pulled")
	c.run(t, "", "copy", "--all", "--src-tls-verify=false", ref, "dir:"+dest)
	getIndex(t, read(t, filepath.Join(dest, "manifest.json")), 2)
	if !bytes.Equal(read(t, filepath.Join(dest, encoded(f.layerDigest))), f.layer) {
		t.Fatal("round-trip layer contents changed")
	}
	for _, instance := range f.instances {
		if !bytes.Equal(read(t, filepath.Join(dest, encoded(instance.Digest)+".manifest.json")), read(t, filepath.Join(f.dir, encoded(instance.Digest)+".manifest.json"))) {
			t.Fatal("round-trip instance digest changed")
		}
	}
	c.run(t, "manifest unknown", "inspect", "--raw", "--tls-verify=false", "docker://"+host+"/team/missing:unknown")
}

func TestPlatformSubsetThroughRegistry(t *testing.T) {
	c, f, host := newCLI(t), makeFixture(t), startRegistry(t)
	src := "docker://" + host + "/team/source:stable"
	c.run(t, "", "copy", "--all", "--dest-tls-verify=false", "dir:"+f.dir, src)
	t.Run("default remains sparse", func(t *testing.T) {
		dest := filepath.Join(t.TempDir(), "sparse")
		c.run(t, "", "copy", "--src-tls-verify=false", "--multi-arch=windows/amd64", src, "dir:"+dest)
		getIndex(t, read(t, filepath.Join(dest, "manifest.json")), 2)
		if _, err := os.Stat(filepath.Join(dest, encoded(f.instances[1].Digest)+".manifest.json")); !os.IsNotExist(err) {
			t.Fatal("unselected instance was copied")
		}
	})
	t.Run("strip and push selected platform", func(t *testing.T) {
		dest := "docker://" + host + "/team/subset:stable"
		c.run(t, "", "copy", "--src-tls-verify=false", "--dest-tls-verify=false", "--multi-arch=windows/amd64", "--strip-removed-platforms", "--remove-list-signatures", src, dest)
		i := getIndex(t, c.run(t, "", "inspect", "--raw", "--tls-verify=false", dest), 1)
		if i.Manifests[0].Digest != f.instances[0].Digest {
			t.Fatal("selected manifest changed")
		}
	})
	t.Run("preserve digests rejects stripping", func(t *testing.T) {
		c.run(t, "preserve", "copy", "--src-tls-verify=false", "--dest-tls-verify=false", "--multi-arch=windows/amd64", "--strip-removed-platforms", "--remove-list-signatures", "--preserve-digests", src, "docker://"+host+"/team/rejected:stable")
	})
}

func TestExecutableSignatureControls(t *testing.T) {
	signature := read(t, filepath.Join("..", "..", "cmd", "skopeo", "fixtures", "image.signature"))
	for _, tc := range []struct {
		name           string
		flags          []string
		list, instance bool
		failure        string
	}{
		{"default preserves signatures", nil, true, true, ""},
		{"remove list only", []string{"--strip-removed-platforms", "--remove-list-signatures"}, false, true, ""},
		{"remove all", []string{"--strip-removed-platforms", "--remove-signatures"}, false, false, ""},
		{"signed list rejects changes", []string{"--strip-removed-platforms"}, false, false, "signature"},
		{"conflicting flags", []string{"--remove-signatures", "--remove-list-signatures"}, false, false, "mutually exclusive"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, f := newCLI(t), makeFixture(t)
			write(t, filepath.Join(f.dir, "signature-1"), signature)
			instanceName := encoded(f.instances[0].Digest) + ".signature-1"
			write(t, filepath.Join(f.dir, instanceName), signature)
			dest := filepath.Join(t.TempDir(), "image")
			args := append([]string{"copy", "--multi-arch=windows/amd64"}, tc.flags...)
			c.run(t, tc.failure, append(args, "dir:"+f.dir, "dir:"+dest)...)
			if tc.failure != "" {
				return
			}
			for name, wanted := range map[string]bool{"signature-1": tc.list, instanceName: tc.instance} {
				data, err := os.ReadFile(filepath.Join(dest, name))
				if wanted {
					if err != nil || !bytes.Equal(data, signature) {
						t.Fatalf("signature %s not preserved: %v", name, err)
					}
				} else if !os.IsNotExist(err) {
					t.Fatalf("signature %s not removed", name)
				}
			}
		})
	}
}

func TestYAMLSyncThroughRegistry(t *testing.T) {
	c, f, host := newCLI(t), makeFixture(t), startRegistry(t)
	c.run(t, "", "copy", "--all", "--dest-tls-verify=false", "dir:"+f.dir, "docker://"+host+"/team/source:stable")
	yamlPath := filepath.Join(t.TempDir(), "sync.yaml")
	write(t, yamlPath, []byte(fmt.Sprintf("%s:\n  tls-verify: false\n  images:\n    team/source: [stable]\n", host)))
	c.run(t, "", "sync", "--all", "--remove-list-signatures", "--src=yaml", "--dest=docker", "--dest-tls-verify=false", yamlPath, host+"/mirror")
	getIndex(t, c.run(t, "", "inspect", "--raw", "--tls-verify=false", "docker://"+host+"/mirror/source:stable"), 2)
}

func foreignOCI(t *testing.T, f fixture, url string) string {
	t.Helper()
	dir := t.TempDir()
	var original manifest
	if err := json.Unmarshal(read(t, filepath.Join(f.dir, encoded(f.instances[0].Digest)+".manifest.json")), &original); err != nil {
		t.Fatal(err)
	}
	config := read(t, filepath.Join(f.dir, encoded(original.Config.Digest)))
	cd := descriptor{MediaType: configType, Digest: hash(config), Size: len(config)}
	write(t, filepath.Join(dir, "blobs", "sha256", encoded(cd.Digest)), config)
	m := encode(t, manifest{2, imageType, cd, []descriptor{{MediaType: foreignType, Digest: f.layerDigest, Size: len(f.layer), URLs: []string{url}}}})
	md := descriptor{MediaType: imageType, Digest: hash(m), Size: len(m)}
	write(t, filepath.Join(dir, "blobs", "sha256", encoded(md.Digest)), m)
	write(t, filepath.Join(dir, "oci-layout"), []byte(`{"imageLayoutVersion":"1.0.0"}`))
	write(t, filepath.Join(dir, "index.json"), encode(t, index{2, indexType, []descriptor{md}}))
	return dir
}

func TestForeignLayerCopyAndSync(t *testing.T) {
	c, f := newCLI(t), makeFixture(t)
	var downloads atomic.Int32
	var broken atomic.Bool
	layers := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downloads.Add(1)
		if broken.Load() {
			http.Error(w, "fixture unavailable", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(f.layer)
	}))
	defer layers.Close()
	source := foreignOCI(t, f, layers.URL+"/layer.tar.gz")
	host := startRegistry(t)
	ref := "docker://" + host + "/team/foreign:stable"
	c.run(t, "", "copy", "--dest-tls-verify=false", "oci:"+source, ref)
	if downloads.Load() != 0 {
		t.Fatal("default copy downloaded foreign layer")
	}
	var copied manifest
	if err := json.Unmarshal(c.run(t, "", "inspect", "--raw", "--tls-verify=false", ref), &copied); err != nil {
		t.Fatal(err)
	}
	if len(copied.Layers) != 1 || len(copied.Layers[0].URLs) != 1 {
		t.Fatal("default lost external layer URL")
	}
	for _, download := range []bool{false, true} {
		destHost := startRegistry(t)
		args := []string{"sync", "--src=docker", "--dest=docker", "--src-tls-verify=false", "--dest-tls-verify=false"}
		if download {
			args = append(args, "--download-foreign-layers")
		}
		c.run(t, "", append(args, host+"/team/foreign:stable", destHost+"/mirror")...)
		if !download && downloads.Load() != 0 {
			t.Fatal("default sync downloaded foreign layer")
		}
		if download {
			if downloads.Load() != 1 {
				t.Fatalf("opt-in sync downloaded layer %d times", downloads.Load())
			}
			broken.Store(true)
			assertRegistryBlob(t, destHost, "mirror/foreign", f)
			// Fetching the layer does not necessarily rewrite its manifest URL.
			// Record the actual executable failure instead of promising offline pulls.
			dest := filepath.Join(t.TempDir(), "external-unavailable")
			c.run(t, "500", "copy", "--src-tls-verify=false", "docker://"+destHost+"/mirror/foreign:stable", "dir:"+dest)
			broken.Store(false)
		}
	}
	copyHost := startRegistry(t)
	beforeCopy := downloads.Load()
	c.run(t, "", "copy", "--download-foreign-layers", "--dest-tls-verify=false", "oci:"+source, "docker://"+copyHost+"/team/foreign:stable")
	if downloads.Load() != beforeCopy+1 {
		t.Fatalf("opt-in copy did not fetch layer: %d requests", downloads.Load())
	}
	broken.Store(true)
	assertRegistryBlob(t, copyHost, "team/foreign", f)
	failureHost := startRegistry(t)
	c.run(t, "failed fetching external blob", "copy", "--download-foreign-layers", "--retry-times=0", "--dest-tls-verify=false", "oci:"+source, "docker://"+failureHost+"/team/failure:stable")
	syncFailureHost := startRegistry(t)
	c.run(t, "500", "sync", "--download-foreign-layers", "--retry-times=0", "--src=docker", "--dest=docker", "--src-tls-verify=false", "--dest-tls-verify=false", host+"/team/foreign:stable", syncFailureHost+"/mirror")
}

// Verify storage through the registry blob endpoint, independently of a
// client's preference for external URLs retained in the manifest.
func assertRegistryBlob(t *testing.T, host, repository string, f fixture) {
	t.Helper()
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://" + host + "/v2/" + repository + "/blobs/" + f.layerDigest)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusOK || !bytes.Equal(b, f.layer) {
		t.Fatalf("downloaded layer not stored in registry: status=%d error=%v digest=%s", resp.StatusCode, err, hash(b))
	}
}
