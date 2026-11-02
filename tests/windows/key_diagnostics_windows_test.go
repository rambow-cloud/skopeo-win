package windows_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Microsoft/go-winio"
)

func TestSigstoreKeyGenerationAndOverwriteProtection(t *testing.T) {
	c := newCLI(t)
	prefix := filepath.Join(t.TempDir(), "key with spaces")
	passphrase := filepath.Join(t.TempDir(), "passphrase")
	write(t, passphrase, []byte("integration-only-passphrase\n"))
	c.run(t, "", "generate-sigstore-key", "--output-prefix", prefix, "--passphrase-file", passphrase)
	pub := read(t, prefix+".pub")
	private := read(t, prefix+".private")
	if !strings.Contains(string(pub), "PUBLIC KEY") || !strings.Contains(string(private), "ENCRYPTED COSIGN PRIVATE KEY") {
		t.Fatal("key pair was not generated")
	}
	c.run(t, "refusing to overwrite", "generate-sigstore-key", "--output-prefix", prefix, "--passphrase-file", passphrase)
	if string(read(t, prefix+".pub")) != string(pub) || string(read(t, prefix+".private")) != string(private) {
		t.Fatal("existing keys were overwritten")
	}
}

func TestPublicKeyWriteFailureIdentifiesPublicKey(t *testing.T) {
	c := newCLI(t)
	prefix := filepath.Join(t.TempDir(), "key")
	pipePath := fmt.Sprintf(`\\.\pipe\skopeo-win-key-%d-%d`, os.Getpid(), time.Now().UnixNano())
	listener, err := winio.ListenPipe(pipePath, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	r := c.execute(t, "error writing public key", func(ctx context.Context) {
		stop := make(chan struct{})
		defer close(stop)
		go func() {
			select {
			case <-ctx.Done():
				_ = listener.Close()
			case <-stop:
			}
		}()
		conn, err := listener.Accept()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = conn.Close() })
		// The CLI opens its passphrase only after checking both output paths.
		// Hold that read, make the public output a directory, then release it.
		// This forces the real write error without sleeps, races, or symlinks.
		if err := os.Mkdir(prefix+".pub", 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Write([]byte("integration-only-passphrase\n")); err != nil {
			t.Fatal(err)
		}
	}, "generate-sigstore-key", "--output-prefix", prefix, "--passphrase-file", pipePath)
	if r.exitCode != 1 || strings.Contains(string(r.stderr), "Error writing private key") {
		t.Fatalf("wrong public-key diagnostic: %s", r.stderr)
	}
	if !strings.Contains(string(read(t, prefix+".private")), "PRIVATE KEY") {
		t.Fatal("private key must have been written before public-key failure")
	}
}
