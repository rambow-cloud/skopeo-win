package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

func writeFeatureJSON(t *testing.T, path string, value any) []byte {
	t.Helper()
	b, err := json.Marshal(value)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, b, 0o600))
	return b
}

// This synthetic directory image has no layers and never executes a container.
func multiPlatformFixture(t *testing.T) (string, []v1.Descriptor) {
	t.Helper()
	dir := t.TempDir()
	var instances []v1.Descriptor
	for _, arch := range []string{"amd64", "arm64"} {
		config, err := json.Marshal(v1.Image{
			Platform: v1.Platform{OS: "windows", Architecture: arch},
			RootFS:   v1.RootFS{Type: "layers", DiffIDs: []digest.Digest{}},
		})
		require.NoError(t, err)
		configDigest := digest.FromBytes(config)
		require.NoError(t, os.WriteFile(filepath.Join(dir, configDigest.Encoded()), config, 0o600))
		m := v1.Manifest{
			Versioned: specs.Versioned{SchemaVersion: 2}, MediaType: v1.MediaTypeImageManifest,
			Config: v1.Descriptor{MediaType: v1.MediaTypeImageConfig, Digest: configDigest, Size: int64(len(config))},
			Layers: []v1.Descriptor{},
		}
		b, err := json.Marshal(m)
		require.NoError(t, err)
		d := digest.FromBytes(b)
		require.NoError(t, os.WriteFile(filepath.Join(dir, d.Encoded()+".manifest.json"), b, 0o600))
		instances = append(instances, v1.Descriptor{
			MediaType: v1.MediaTypeImageManifest, Digest: d, Size: int64(len(b)),
			Platform: &v1.Platform{OS: "windows", Architecture: arch},
		})
	}
	writeFeatureJSON(t, filepath.Join(dir, "manifest.json"), v1.Index{
		Versioned: specs.Versioned{SchemaVersion: 2}, MediaType: v1.MediaTypeImageIndex, Manifests: instances,
	})
	return dir, instances
}

func TestSelectedPlatformCopies(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		flags                 []string
		signed                bool
		wantCount             int
		wantInstanceSignature bool
		wantListSignature     bool
		wantError             string
	}{
		{name: "default sparse list", wantCount: 2},
		{name: "default preserves signatures", signed: true, wantCount: 2, wantInstanceSignature: true, wantListSignature: true},
		{name: "strip list", flags: []string{"--strip-removed-platforms", "--remove-list-signatures"}, wantCount: 1},
		{name: "retain instance signature", flags: []string{"--strip-removed-platforms", "--remove-list-signatures"}, signed: true, wantCount: 1, wantInstanceSignature: true},
		{name: "remove all signatures", flags: []string{"--strip-removed-platforms", "--remove-signatures"}, signed: true, wantCount: 1},
		{name: "signed list needs removal", flags: []string{"--strip-removed-platforms"}, signed: true, wantError: "signature"},
		{name: "preserve digest prevents stripping", flags: []string{"--strip-removed-platforms", "--remove-list-signatures", "--preserve-digests"}, wantError: "preserve"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src, instances := multiPlatformFixture(t)
			// A real signature packet is preserved as opaque data under the explicitly
			// insecure test policy; trust verification is covered separately.
			sig, err := os.ReadFile("fixtures/image.signature")
			require.NoError(t, err)
			if tc.signed {
				require.NoError(t, os.WriteFile(filepath.Join(src, "signature-1"), sig, 0o600))
				require.NoError(t, os.WriteFile(filepath.Join(src, instances[0].Digest.Encoded()+".signature-1"), sig, 0o600))
			}
			dest := filepath.Join(t.TempDir(), "image")
			args := append([]string{"--insecure-policy", "copy", "--multi-arch=windows/amd64"}, tc.flags...)
			_, err = runSkopeo(append(args, "dir:"+src, "dir:"+dest)...)
			if tc.wantError != "" {
				require.ErrorContains(t, err, tc.wantError)
				return
			}
			require.NoError(t, err)
			b, err := os.ReadFile(filepath.Join(dest, "manifest.json"))
			require.NoError(t, err)
			var index v1.Index
			require.NoError(t, json.Unmarshal(b, &index))
			require.Len(t, index.Manifests, tc.wantCount)
			assert.Equal(t, instances[0].Digest, index.Manifests[0].Digest)
			_, err = os.Stat(filepath.Join(dest, instances[1].Digest.Encoded()+".manifest.json"))
			assert.True(t, os.IsNotExist(err), "unselected image must not be copied")
			listSig, err := os.ReadFile(filepath.Join(dest, "signature-1"))
			if tc.wantListSignature {
				require.NoError(t, err)
				assert.Equal(t, sig, listSig)
			} else {
				assert.True(t, os.IsNotExist(err))
			}
			b, err = os.ReadFile(filepath.Join(dest, instances[0].Digest.Encoded()+".signature-1"))
			if tc.wantInstanceSignature {
				require.NoError(t, err)
				assert.Equal(t, sig, b)
			} else {
				assert.True(t, os.IsNotExist(err))
			}
		})
	}
}

func TestForeignLayerDownload(t *testing.T) {
	rawLayer := []byte("synthetic foreign layer\n")
	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	_, err := gz.Write(rawLayer)
	require.NoError(t, err)
	require.NoError(t, gz.Close())
	layer := compressed.Bytes()
	layerDigest := digest.FromBytes(layer)
	var downloads atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downloads.Add(1)
		_, _ = w.Write(layer)
	}))
	defer server.Close()
	src := t.TempDir()
	writeFeatureJSON(t, filepath.Join(src, "oci-layout"), v1.ImageLayout{Version: "1.0.0"})
	writeBlob := func(value any, mediaType string) v1.Descriptor {
		b, err := json.Marshal(value)
		require.NoError(t, err)
		d := digest.FromBytes(b)
		path := filepath.Join(src, "blobs", "sha256", d.Encoded())
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, b, 0o600))
		return v1.Descriptor{MediaType: mediaType, Digest: d, Size: int64(len(b))}
	}
	config := writeBlob(v1.Image{
		Platform: v1.Platform{OS: "windows", Architecture: "amd64"},
		RootFS:   v1.RootFS{Type: "layers", DiffIDs: []digest.Digest{digest.FromBytes(rawLayer)}},
	}, v1.MediaTypeImageConfig)
	m := writeBlob(v1.Manifest{
		Versioned: specs.Versioned{SchemaVersion: 2}, MediaType: v1.MediaTypeImageManifest, Config: config,
		Layers: []v1.Descriptor{{MediaType: v1.MediaTypeImageLayerNonDistributableGzip, Digest: layerDigest, Size: int64(len(layer)), URLs: []string{server.URL + "/layer"}}},
	}, v1.MediaTypeImageManifest)
	writeFeatureJSON(t, filepath.Join(src, "index.json"), v1.Index{Versioned: specs.Versioned{SchemaVersion: 2}, Manifests: []v1.Descriptor{m}})
	for _, download := range []bool{false, true} {
		dest := filepath.Join(t.TempDir(), "image")
		args := []string{"--insecure-policy", "copy"}
		if download {
			args = append(args, "--download-foreign-layers")
		}
		_, err := runSkopeo(append(args, "oci:"+src, "oci:"+dest)...)
		require.NoError(t, err)
		rawManifest, err := runSkopeo("inspect", "--raw", "oci:"+dest)
		require.NoError(t, err)
		var copiedManifest v1.Manifest
		require.NoError(t, json.Unmarshal([]byte(rawManifest), &copiedManifest))
		require.Len(t, copiedManifest.Layers, 1)
		assert.Equal(t, layerDigest, copiedManifest.Layers[0].Digest)
		b, err := os.ReadFile(filepath.Join(dest, "blobs", "sha256", layerDigest.Encoded()))
		if download {
			require.NoError(t, err)
			assert.Equal(t, layer, b)
			assert.EqualValues(t, 1, downloads.Load())
		} else {
			assert.Equal(t, []string{server.URL + "/layer"}, copiedManifest.Layers[0].URLs)
			assert.True(t, os.IsNotExist(err))
			assert.Zero(t, downloads.Load())
		}
	}
}

func TestNewCopyFlagsSharedWithSync(t *testing.T) {
	for _, name := range []string{"copy", "sync"} {
		app, _ := createApp()
		cmd, _, err := app.Find([]string{name})
		require.NoError(t, err)
		require.NotNil(t, cmd.Flags().Lookup("download-foreign-layers"))
		require.NotNil(t, cmd.Flags().Lookup("remove-list-signatures"))
	}
}

func TestArgumentErrorsShowHelp(t *testing.T) {
	for _, name := range []string{"delete", "inspect", "layers", "manifest-digest", "standalone-sign", "standalone-verify", "untrusted-signature-dump-without-verification", "generate-sigstore-key"} {
		t.Run(name, func(t *testing.T) {
			app, _ := createApp()
			var stdout, stderr bytes.Buffer
			app.SetOut(&stdout)
			app.SetErr(&stderr)
			app.SetArgs([]string{name})
			require.Error(t, app.Execute())
			assert.Empty(t, stdout.String())
			assert.Contains(t, stderr.String(), "Usage:")
			assert.Contains(t, stderr.String(), name)
		})
	}
}

func TestSyncMaintainedYAMLCompatibility(t *testing.T) {
	var cfg sourceConfig
	require.NoError(t, yaml.Unmarshal([]byte(`registry.example.com:
  tls-verify: false
  images:
    team/app: &tags [stable, "1.0"]
    team/other: *tags
  images-by-tag-regex:
    team/filtered: '^v[0-9]+$'
  credentials:
    username: test-user
    password: test-password
`), &cfg))
	registry := cfg["registry.example.com"]
	assert.Equal(t, []string{"stable", "1.0"}, registry.Images["team/other"])
	assert.Equal(t, registry.Images["team/app"], registry.Images["team/other"])
	assert.Equal(t, "test-user", registry.Credentials.Username)
	assert.Equal(t, "^v[0-9]+$", registry.ImagesByTagRegex["team/filtered"])
}
