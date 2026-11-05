package windows_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/registry"
)

func TestNewCLIArgumentDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []string
		message string
	}{
		{"delete", []string{"delete"}, "exactly one argument expected"},
		{"inspect", []string{"inspect"}, "exactly one argument expected"},
		{"layers", []string{"layers"}, "at least one argument expected"},
		{"manifest-digest", []string{"manifest-digest"}, "exactly one argument expected"},
		{"standalone-sign arguments", []string{"standalone-sign"}, "exactly three arguments expected"},
		{"standalone-sign output", []string{"standalone-sign", "manifest", "example.com/image", "fingerprint"}, "--output must be specified"},
		{"standalone-verify", []string{"standalone-verify"}, "exactly four arguments expected"},
		{"untrusted signature dump", []string{"untrusted-signature-dump-without-verification"}, "exactly one argument expected"},
		{"key prefix", []string{"generate-sigstore-key"}, "--output-prefix must be specified"},
		{"key arguments", []string{"generate-sigstore-key", "--output-prefix", "unused", "unexpected"}, "no arguments expected"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newCLI(t)
			r := c.execute(t, tc.message, nil, tc.args...)
			if r.exitCode != 1 || len(r.stdout) != 0 || !strings.Contains(string(r.stderr), "Usage:") || !strings.Contains(string(r.stderr), tc.args[0]) {
				t.Fatalf("invalid diagnostics: code=%d stdout=%s stderr=%s", r.exitCode, r.stdout, r.stderr)
			}
		})
	}
}

func TestDefaultRootHelp(t *testing.T) {
	c := newCLI(t)
	help := c.executeExact(t, "", nil, "--help")
	if help.exitCode != 0 || len(help.stderr) != 0 || !bytes.Contains(help.stdout, []byte("Available Commands:")) {
		t.Fatalf("invalid explicit help: %+v", help)
	}
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"no arguments", nil},
		{"global flag", []string{"--debug"}},
		{"global flag value", []string{"--command-timeout", "30s"}},
		{"argument terminator", []string{"--"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := c.executeExact(t, "", nil, tc.args...)
			if r.exitCode != 0 || len(r.stderr) != 0 || !bytes.Equal(r.stdout, help.stdout) {
				t.Fatalf("default invocation differs from --help: code=%d stdout=%s stderr=%s", r.exitCode, r.stdout, r.stderr)
			}
			assertNoDefaultPolicy(t, c.root)
		})
	}
	for _, tc := range []struct {
		name, message string
		args          []string
	}{
		{"unknown command", "Unrecognized command", []string{"inspec"}},
		{"unknown flag", "unknown flag", []string{"--unknown-flag"}},
		{"conflicting flags", "mutually exclusive", []string{"--insecure-policy", "--require-signed"}},
		{"missing inspect argument", "exactly one argument expected", []string{"inspect"}},
		{"missing copy arguments", "exactly two arguments expected", []string{"copy"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := c.executeExact(t, tc.message, nil, tc.args...)
			if r.exitCode != 1 || len(r.stdout) != 0 {
				t.Fatalf("invalid command succeeded or printed help to stdout: code=%d stdout=%s stderr=%s", r.exitCode, r.stdout, r.stderr)
			}
			assertNoDefaultPolicy(t, c.root)
		})
	}
}

func TestDurationAndCrossPlatformHelp(t *testing.T) {
	c := newCLI(t)
	for _, args := range [][]string{{"--help"}, {"inspect", "--help"}} {
		r := c.execute(t, "", nil, args...)
		if len(r.stderr) != 0 {
			t.Fatalf("help wrote to stderr: %s", r.stderr)
		}
		fragments := []string{"--override-os", "--override-arch"}
		if args[0] == "--help" {
			fragments = append(fragments, "30s, 10m")
		}
		for _, fragment := range fragments {
			if !strings.Contains(string(r.stdout), fragment) {
				t.Fatalf("help missing %q", fragment)
			}
		}
		if args[0] == "inspect" && !strings.Contains(string(r.stdout), "--override-os linux --override-arch amd64") {
			t.Fatal("inspect override example missing")
		}
	}
	for _, duration := range []string{"30s", "10m"} {
		c.run(t, "", "--command-timeout="+duration, "--version")
	}
	c.run(t, "invalid duration", "--command-timeout=invalid", "--version")
	for _, command := range []string{"copy", "sync"} {
		output := string(c.run(t, "", command, "--help"))
		for _, flag := range []string{"--remove-list-signatures", "--download-foreign-layers"} {
			if !strings.Contains(output, flag) {
				t.Fatalf("%s help missing %s", command, flag)
			}
		}
		if command == "copy" && !strings.Contains(output, "--strip-removed-platforms") {
			t.Fatal("copy help missing strip option")
		}
	}
}

func singleImageFixture(t *testing.T, f fixture) string {
	t.Helper()
	dir := t.TempDir()
	var m manifest
	b := read(t, filepath.Join(f.dir, encoded(f.instances[0].Digest)+".manifest.json"))
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "manifest.json"), b)
	write(t, filepath.Join(dir, encoded(m.Config.Digest)), read(t, filepath.Join(f.dir, encoded(m.Config.Digest))))
	write(t, filepath.Join(dir, encoded(f.layerDigest)), f.layer)
	return dir
}

func TestCopyPlatformAndSignatureBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name    string
		flags   []string
		single  bool
		failure string
		count   int
	}{
		{"strip unsigned without removal flag", []string{"--multi-arch=windows/amd64", "--strip-removed-platforms"}, false, "", 1},
		{"keep both requested platforms", []string{"--multi-arch=windows/amd64,windows/arm64", "--strip-removed-platforms", "--remove-list-signatures"}, false, "", 2},
		{"preserve sparse index digest", []string{"--multi-arch=windows/amd64", "--preserve-digests"}, false, "", 2},
		{"all conflicts with platform selection", []string{"--all", "--multi-arch=windows/amd64"}, false, "cannot use --all", 0},
		{"single image rejects list signature removal", []string{"--remove-list-signatures"}, true, "not applicable", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, f := newCLI(t), makeFixture(t)
			src := f.dir
			if tc.single {
				src = singleImageFixture(t, f)
			}
			dest := filepath.Join(t.TempDir(), "image")
			args := append([]string{"copy"}, tc.flags...)
			c.run(t, tc.failure, append(args, "dir:"+src, "dir:"+dest)...)
			if tc.failure != "" {
				return
			}
			b := read(t, filepath.Join(dest, "manifest.json"))
			i := getIndex(t, b, tc.count)
			if i.Manifests[0].Digest != f.instances[0].Digest {
				t.Fatal("selected instance changed")
			}
			if tc.name == "preserve sparse index digest" && hash(b) != hash(read(t, filepath.Join(src, "manifest.json"))) {
				t.Fatal("preserve-digests changed the sparse index")
			}
		})
	}
}

func TestSyncSignatureControls(t *testing.T) {
	sig := read(t, filepath.Join("..", "..", "cmd", "skopeo", "fixtures", "image.signature"))
	for _, tc := range []struct {
		name           string
		flags          []string
		single         bool
		failure        string
		list, instance bool
	}{
		{"default preserves list and instances", []string{"--all"}, false, "", true, true},
		{"remove list and preserve both instances", []string{"--all", "--remove-list-signatures"}, false, "", false, true},
		{"remove all signatures", []string{"--all", "--remove-signatures"}, false, "", false, false},
		{"reject conflicting removal flags", []string{"--all", "--remove-list-signatures", "--remove-signatures"}, false, "mutually exclusive", false, false},
		{"reject list removal on single image", []string{"--remove-list-signatures"}, true, "not applicable", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, f := newCLI(t), makeFixture(t)
			src := f.dir
			if tc.single {
				src = singleImageFixture(t, f)
			}
			write(t, filepath.Join(src, "signature-1"), sig)
			for _, d := range f.instances {
				write(t, filepath.Join(src, encoded(d.Digest)+".signature-1"), sig)
			}
			// Registry signature lookaside storage uses a local directory to
			// verify preservation independently of registry signature extensions.
			host := startRegistry(t)
			lookaside := filepath.Join(t.TempDir(), "signatures")
			if filepath.VolumeName(lookaside) != filepath.VolumeName(c.root) {
				t.Fatal("signature fixture and CLI working directory must share a drive")
			}
			// The library passes file-URL paths directly to os.Open. A root-relative
			// Windows path on the CLI's current drive resolves inside this temp dir.
			lookasideURL := "file://" + filepath.ToSlash(strings.TrimPrefix(lookaside, filepath.VolumeName(lookaside)))
			config := fmt.Sprintf("docker:\n  %s:\n    lookaside: %q\n    lookaside-staging: %q\n", host+"/team", lookasideURL, lookasideURL)
			write(t, filepath.Join(c.root, "registries.d", "signatures.yaml"), []byte(config))
			destRepo := host + "/team"
			// Give the directory a stable tagged basename for sync destinations.
			input := filepath.Join(t.TempDir(), "synced")
			copyFixtureDirectory(t, src, input)
			args := append([]string{"sync", "--src=dir", "--dest=docker", "--dest-tls-verify=false"}, tc.flags...)
			c.run(t, tc.failure, append(args, input, destRepo)...)
			if tc.failure != "" {
				return
			}
			output := filepath.Join(t.TempDir(), "roundtrip")
			pull := []string{"copy", "--src-tls-verify=false"}
			if !tc.single {
				pull = append(pull, "--all")
			}
			c.run(t, "", append(pull, "docker://"+host+"/team/synced:latest", "dir:"+output)...)
			for name, want := range map[string]bool{"signature-1": tc.list, encoded(f.instances[0].Digest) + ".signature-1": tc.instance, encoded(f.instances[1].Digest) + ".signature-1": tc.instance} {
				b, err := os.ReadFile(filepath.Join(output, name))
				if want {
					if err != nil || !bytes.Equal(b, sig) {
						t.Fatalf("signature %s not preserved: %v", name, err)
					}
				} else if !os.IsNotExist(err) {
					t.Fatalf("signature %s not removed", name)
				}
			}
		})
	}
}

func copyFixtureDirectory(t *testing.T, src, dest string) {
	t.Helper()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			write(t, filepath.Join(dest, entry.Name()), read(t, filepath.Join(src, entry.Name())))
		}
	}
}

func TestYAMLCompatibilityAndAuthentication(t *testing.T) {
	c, f := newCLI(t), makeFixture(t)
	var accepted atomic.Int32
	backend := registry.New(registry.Logger(log.New(io.Discard, "", 0)))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, password, ok := r.BasicAuth()
		if !ok || user != "fixture-user" || password != "fixture-password" {
			w.Header().Set("WWW-Authenticate", `Basic realm="integration"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		accepted.Add(1)
		backend.ServeHTTP(w, r)
	}))
	defer server.Close()
	host := strings.TrimPrefix(server.URL, "http://")
	for _, repo := range []string{"source", "other", "filtered"} {
		for _, tag := range []string{"stable", "ignored"} {
			c.run(t, "", "copy", "--all", "--dest-tls-verify=false", "--dest-creds=fixture-user:fixture-password", "dir:"+f.dir, "docker://"+host+"/team/"+repo+":"+tag)
		}
	}
	config := fmt.Sprintf(`%s:
  tls-verify: false
  credentials:
    username: fixture-user
    password: fixture-password
  images:
    team/source: &tags [stable]
    team/other: *tags
  images-by-tag-regex:
    team/filtered: '^stable$'
`, host)
	path := filepath.Join(t.TempDir(), "source.yaml")
	write(t, path, []byte(config))
	destHost := startRegistry(t)
	// Explicit YAML false must override the conflicting true CLI flag.
	c.run(t, "", "sync", "--all", "--src=yaml", "--dest=docker", "--src-tls-verify=true", "--dest-tls-verify=false", path, destHost+"/mirror")
	if accepted.Load() == 0 {
		t.Fatal("YAML credentials did not authenticate")
	}
	for _, repo := range []string{"source", "other", "filtered"} {
		getIndex(t, c.run(t, "", "inspect", "--raw", "--tls-verify=false", "docker://"+destHost+"/mirror/"+repo+":stable"), 2)
		var tags struct {
			Tags []string `json:"Tags"`
		}
		if err := json.Unmarshal(c.run(t, "", "list-tags", "--tls-verify=false", "docker://"+destHost+"/mirror/"+repo), &tags); err != nil {
			t.Fatal(err)
		}
		if len(tags.Tags) != 1 || tags.Tags[0] != "stable" {
			t.Fatalf("YAML filter copied unexpected tags: %v", tags.Tags)
		}
	}
	t.Run("invalid credentials fail", func(t *testing.T) {
		bad := filepath.Join(t.TempDir(), "bad-auth.yaml")
		// Explicit image selection ensures authentication failures cannot be
		// discarded as an empty regex-discovery result.
		write(t, bad, []byte(fmt.Sprintf("%s:\n  tls-verify: false\n  credentials:\n    username: fixture-user\n    password: wrong\n  images:\n    team/source: [stable]\n", host)))
		c.run(t, "authentication required", "sync", "--all", "--src=yaml", "--dest=docker", "--dest-tls-verify=false", bad, destHost+"/rejected")
	})
	for _, tc := range []struct{ name, value string }{{"invalid YAML", "[broken"}, {"invalid TLS boolean", fmt.Sprintf("%s:\n  tls-verify: invalid-boolean\n", host)}} {
		t.Run(tc.name, func(t *testing.T) {
			bad := filepath.Join(t.TempDir(), "invalid.yaml")
			write(t, bad, []byte(tc.value))
			c.run(t, "unmarshal", "sync", "--src=yaml", "--dest=docker", bad, destHost+"/rejected")
		})
	}
}

func TestCommandTimeoutIsEnforced(t *testing.T) {
	var contacted atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contacted.Add(1)
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}))
	defer s.Close()
	c := newCLI(t)
	c.run(t, "context deadline exceeded", "--command-timeout=1s", "inspect", "--tls-verify=false", "docker://"+strings.TrimPrefix(s.URL, "http://")+"/team/image:stable")
	if contacted.Load() == 0 {
		t.Fatal("timeout fixture was never contacted")
	}
}
