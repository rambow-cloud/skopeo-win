package windows_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestInvalidArgumentDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		name, message string
		args          []string
	}{
		{"unknown subcommand flag", "unknown flag: --unknown-option", []string{"inspect", "--unknown-option"}},
		{"unknown shorthand", "unknown shorthand flag", []string{"-Z"}},
		{"missing global flag value", "flag needs an argument: --override-os", []string{"--override-os"}},
		{"missing timeout", "flag needs an argument: --command-timeout", []string{"--command-timeout"}},
		{"missing retry count", "flag needs an argument: --retry-times", []string{"inspect", "--retry-times"}},
		{"missing retry delay", "flag needs an argument: --retry-delay", []string{"inspect", "--retry-delay"}},
		{"invalid Boolean", "--tls-verify", []string{"inspect", "--tls-verify=perhaps"}},
		{"invalid integer", "--retry-times", []string{"inspect", "--retry-times=abc"}},
		{"invalid retry duration", "--retry-delay", []string{"inspect", "--retry-delay=abc"}},
		{"invalid timeout", "--command-timeout", []string{"--command-timeout=abc"}},
		{"negative unsigned value", "--image-parallel-copies", []string{"copy", "--image-parallel-copies=-1"}},
		{"invalid compression level", "--dest-compress-level", []string{"copy", "--dest-compress-level=abc"}},
		{"extra inspect argument", "exactly one argument expected", []string{"inspect", "one", "two"}},
		{"extra copy argument", "exactly two arguments expected", []string{"copy", "one", "two", "three"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newCLI(t)
			r := c.executeExact(t, tc.message, nil, tc.args...)
			if r.exitCode != 1 || len(r.stdout) != 0 || strings.Contains(string(r.stderr), "panic:") {
				t.Fatalf("invalid diagnostics: exit=%d stdout=%s stderr=%s", r.exitCode, r.stdout, r.stderr)
			}
			assertNoDefaultPolicy(t, c.root)
		})
	}
	c := newCLI(t)
	c.run(t, "unknown multi-arch option", "copy", "--multi-arch=invalid", "dir:"+c.root, "dir:"+filepath.Join(c.root, "out"))
}

func TestNegativeRetryOptionsBeforeHelp(t *testing.T) {
	for _, command := range []string{"copy", "sync", "inspect", "delete", "layers", "list-tags"} {
		for _, flag := range []string{"--retry-times=-1", "--retry-delay=-1s"} {
			t.Run(command+"/"+flag, func(t *testing.T) {
				c := newCLI(t)
				r := c.executeExact(t, "must not be negative", nil, command, flag, "--help")
				if r.exitCode != 1 || len(r.stdout) != 0 || !strings.Contains(string(r.stderr), strings.Split(flag, "=")[0]) {
					t.Fatalf("negative option lacks its flag diagnostic: %+v", r)
				}
			})
		}
	}
}

func TestNumericOptionValidationBeforeSideEffects(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Error(w, "fixture unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	for _, tc := range []struct {
		name       string
		flags      []string
		diagnostic string
	}{
		{"negative timeout", []string{"--command-timeout=-1s"}, "--command-timeout"},
		{"negative retry count", []string{"--retry-times=-1"}, "--retry-times"},
		{"negative retry delay", []string{"--retry-times=1", "--retry-delay=-1s"}, "--retry-delay"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newCLI(t)
			c.defaultPolicy, c.imageOS = true, ""
			ref := publicMirror(t, c, "docker.io/library", strings.TrimPrefix(server.URL, "http://"), true)
			dest, digest := filepath.Join(c.root, "out"), filepath.Join(c.root, "digest.txt")
			args := append([]string{"copy"}, tc.flags...)
			args = append(args, "--digestfile", digest, ref, "dir:"+dest)
			r := c.execute(t, "must not be negative", nil, args...)
			if r.exitCode != 1 || len(r.stdout) != 0 || !strings.Contains(string(r.stderr), tc.diagnostic) || strings.Contains(string(r.stderr), "panic:") {
				t.Fatalf("wrong range diagnostic: %+v", r)
			}
			assertNoDefaultPolicy(t, c.root)
			for _, path := range []string{dest, digest} {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("invalid flags created output: %s / %v", path, err)
				}
			}
			if requests.Load() != 0 {
				t.Fatal("invalid options reached the registry")
			}
		})
	}
	for _, delay := range []string{"1ns", "9ns"} {
		c := newCLI(t)
		r := c.executeExact(t, "at least 10ns", nil, "inspect", "--retry-delay="+delay, "--help")
		if r.exitCode != 1 || len(r.stdout) != 0 || !strings.Contains(string(r.stderr), "--retry-delay") {
			t.Fatal("tiny retry delay lacks an actionable range error")
		}
	}
	// A real operation with valid values must reach the independent server.
	// This prevents a no-op/error-only implementation from satisfying the checks.
	c := newCLI(t)
	c.execute(t, "503", nil, "--command-timeout=5s", "inspect", "--retry-times=1", "--retry-delay=1ms", "--tls-verify=false", "docker://"+strings.TrimPrefix(server.URL, "http://")+"/image:latest")
	if requests.Load() < 2 {
		t.Fatal("valid retry options did not execute the registry operation")
	}
	c.execute(t, "503", nil, "--command-timeout=5s", "inspect", "--retry-times=1", "--retry-delay=10ns", "--tls-verify=false", "docker://"+strings.TrimPrefix(server.URL, "http://")+"/image:latest")
	r := c.executeExact(t, "", nil, "--command-timeout=0s", "inspect", "--retry-times=0", "--retry-delay=0s", "--help")
	if r.exitCode != 0 || len(r.stderr) != 0 {
		t.Fatal("valid zero options no longer display help")
	}
}
