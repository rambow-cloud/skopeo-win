package windows_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestSignatureRemovalDoesNotBypassImageTrustPolicy(t *testing.T) {
	for _, command := range []string{"copy", "sync"} {
		for _, flag := range []string{"--remove-list-signatures", "--remove-signatures", "--download-foreign-layers"} {
			t.Run(command+"/"+flag, func(t *testing.T) {
				c, f := newCLI(t), makeFixture(t)
				// A positive control proves this fixture can actually be copied.
				c.run(t, "", "copy", "--all", "dir:"+f.dir, "dir:"+filepath.Join(t.TempDir(), "control"))
				policy := filepath.Join(c.root, "reject-policy.json")
				write(t, policy, []byte(`{"default":[{"type":"reject"}]}`))
				c.policy = policy
				args := []string{command, "--all", flag}
				if command == "copy" {
					args = append(args, "dir:"+f.dir, "dir:"+filepath.Join(t.TempDir(), "rejected"))
				} else {
					host := startRegistry(t)
					args = append(args, "--src=dir", "--dest=docker", "--dest-tls-verify=false", f.dir, host+"/rejected")
				}
				r := c.execute(t, "rejected", nil, args...)
				if r.exitCode != 1 {
					t.Fatalf("policy rejection returned %d", r.exitCode)
				}
			})
		}
	}
	t.Run("stripping is still subject to policy", func(t *testing.T) {
		c, f := newCLI(t), makeFixture(t)
		c.policy = filepath.Join(c.root, "reject.json")
		write(t, c.policy, []byte(`{"default":[{"type":"reject"}]}`))
		c.run(t, "rejected", "copy", "--multi-arch=windows/amd64", "--strip-removed-platforms", "--remove-list-signatures", "dir:"+f.dir, "dir:"+filepath.Join(t.TempDir(), "rejected"))
	})
}

func TestRejectedImageCannotTriggerForeignLayerDownload(t *testing.T) {
	c, f := newCLI(t), makeFixture(t)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); _, _ = w.Write(f.layer) }))
	defer server.Close()
	source := foreignOCI(t, f, server.URL+"/layer")
	c.policy = filepath.Join(c.root, "reject.json")
	write(t, c.policy, []byte(`{"default":[{"type":"reject"}]}`))
	c.run(t, "rejected", "copy", "--download-foreign-layers", "oci:"+source, "oci:"+filepath.Join(t.TempDir(), "rejected"))
	if requests.Load() != 0 {
		t.Fatal("rejected image triggered an external layer request")
	}
}

func TestStandaloneVerificationEnforcesTrustAndIntegrity(t *testing.T) {
	fixtureRoot, err := filepath.Abs(filepath.Join("..", "..", "cmd", "skopeo", "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	publicKey := filepath.Join(fixtureRoot, "pubring.gpg")
	manifestPath := filepath.Join(fixtureRoot, "image.manifest.json")
	signaturePath, err := filepath.Abs(filepath.Join("fixtures", "image-uncompressed.signature"))
	if err != nil {
		t.Fatal(err)
	}
	const fingerprint = "08CD26E446E2E95249B7A405E932F44B23E8DD43"
	c := newCLI(t)
	verify := func(t *testing.T, message, manifest, signature, identity, key string) []byte {
		t.Helper()
		return c.run(t, message, "standalone-verify", "--public-key-file", publicKey, manifest, identity, key, signature)
	}
	if output := verify(t, "", manifestPath, signaturePath, "testing/manifest", fingerprint); !strings.Contains(string(output), "Signature verified using fingerprint "+fingerprint) {
		t.Fatal("valid trusted fixture did not verify")
	}
	for _, name := range []string{"manifest changed", "signature changed", "wrong identity", "wrong fingerprint", "unsigned payload"} {
		t.Run(name, func(t *testing.T) {
			mp, sp, identity, key := manifestPath, signaturePath, "testing/manifest", fingerprint
			switch name {
			case "manifest changed":
				mp = filepath.Join(t.TempDir(), "manifest.json")
				write(t, mp, append(bytes.Clone(read(t, manifestPath)), '\n'))
			case "signature changed":
				sp = filepath.Join(t.TempDir(), "signature")
				b := bytes.Clone(read(t, signaturePath))
				b[len(b)-1] ^= 0xff
				write(t, sp, b)
			case "wrong identity":
				identity = "different/image"
			case "wrong fingerprint":
				key = "0123456789ABCDEF0123456789ABCDEF01234567"
			case "unsigned payload":
				sp = filepath.Join(t.TempDir(), "unsigned")
				write(t, sp, []byte("unsigned payload"))
			}
			if output := verify(t, "Error verifying signature", mp, sp, identity, key); len(output) != 0 {
				t.Fatal("rejected signature produced success output")
			}
		})
	}
}
