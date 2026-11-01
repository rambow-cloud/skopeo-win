//go:build containers_image_openpgp

package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/stretchr/testify/require"
	"go.podman.io/image/v5/signature"
)

func TestPureGoOpenPGPVerification(t *testing.T) {
	keyCreated := time.Now().Add(-24 * time.Hour)
	key, err := openpgp.NewEntity("Skopeo test", "", "test@example.invalid", &packet.Config{
		RSABits: 2048,
		Time:    func() time.Time { return keyCreated },
	})
	require.NoError(t, err)
	var publicKey bytes.Buffer
	require.NoError(t, key.Serialize(&publicKey))
	mechanism, identities, err := signature.NewEphemeralGPGSigningMechanism(publicKey.Bytes())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, mechanism.Close()) })
	require.Equal(t, []string{strings.ToUpper(fmt.Sprintf("%x", key.PrimaryKey.Fingerprint))}, identities)

	sign := func(payload []byte, config *packet.Config) []byte {
		t.Helper()
		var out bytes.Buffer
		writer, err := openpgp.Sign(&out, key, nil, config)
		require.NoError(t, err)
		_, err = writer.Write(payload)
		require.NoError(t, err)
		require.NoError(t, writer.Close())
		return out.Bytes()
	}

	payload := []byte("skopeo signature verification regression payload")
	signed := sign(payload, nil)
	verified, identity, err := mechanism.Verify(signed)
	require.NoError(t, err)
	require.Equal(t, payload, verified)
	require.Equal(t, identities[0], identity)

	t.Run("tampered signature", func(t *testing.T) {
		tampered := bytes.Clone(signed)
		tampered[len(tampered)-1] ^= 0xff
		_, _, err := mechanism.Verify(tampered)
		require.Error(t, err)
	})
	t.Run("unsigned input", func(t *testing.T) {
		_, _, err := mechanism.Verify(payload)
		require.Error(t, err)
	})
	t.Run("untrusted signer", func(t *testing.T) {
		t.Setenv("GNUPGHOME", t.TempDir())
		untrusted, err := signature.NewGPGSigningMechanism()
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, untrusted.Close()) })
		_, _, err = untrusted.Verify(signed)
		require.Error(t, err)
	})
	t.Run("expired signature", func(t *testing.T) {
		created := time.Now().Add(-time.Hour)
		expired := sign(payload, &packet.Config{Time: func() time.Time { return created }, SigLifetimeSecs: 1})
		_, _, err := mechanism.Verify(expired)
		require.Error(t, err)
	})
}
