// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ssh

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"gitea.dev/modules/generate"

	"github.com/gliderlabs/ssh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
)

func TestGenKeyPair(t *testing.T) {
	testCases := []struct {
		keyType      generate.SSHKeyType
		expectedType any
	}{
		{
			keyType:      generate.SSHKeyRSA,
			expectedType: &rsa.PrivateKey{},
		},
		{
			keyType:      generate.SSHKeyED25519,
			expectedType: &ed25519.PrivateKey{},
		},
		{
			keyType:      generate.SSHKeyECDSA,
			expectedType: &ecdsa.PrivateKey{},
		},
	}
	tmpDir := t.TempDir()
	for _, tc := range testCases {
		name := "gitea." + string(tc.keyType)
		fn := filepath.Join(tmpDir, name)
		t.Run("Generate "+name, func(t *testing.T) {
			require.NoError(t, GenKeyPair(fn, tc.keyType, 0))

			bytes, err := os.ReadFile(fn)
			require.NoError(t, err)

			privateKey, err := gossh.ParseRawPrivateKey(bytes)
			require.NoError(t, err)
			assert.IsType(t, tc.expectedType, privateKey)
		})
	}
	t.Run("Generate unknown key type", func(t *testing.T) {
		err := GenKeyPair(t.TempDir()+"gitea.badkey", "badkey", 0)
		require.Error(t, err)
	})
}

func TestAddHostKey_RSARestrictedToSHA2(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "rsa_host_key")

	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(rsaKey)
	require.NoError(t, err)
	f, err := os.OpenFile(keyPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	require.NoError(t, err)
	require.NoError(t, pem.Encode(f, &pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8Bytes}))
	require.NoError(t, f.Close())

	srv := &ssh.Server{}
	require.NoError(t, addHostKey(srv, keyPath))
	require.Len(t, srv.HostSigners, 1)

	mas, ok := srv.HostSigners[0].(gossh.MultiAlgorithmSigner)
	require.True(t, ok, "expected MultiAlgorithmSigner for RSA key")

	algos := mas.Algorithms()
	assert.Contains(t, algos, gossh.KeyAlgoRSASHA256)
	assert.Contains(t, algos, gossh.KeyAlgoRSASHA512)
	assert.NotContains(t, algos, gossh.KeyAlgoRSA, "ssh-rsa (SHA-1) must not be advertised")
}

func TestAddHostKey_Ed25519NotRestricted(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "ed25519_host_key")
	require.NoError(t, GenKeyPair(keyPath, generate.SSHKeyED25519, 0))

	srv := &ssh.Server{}
	require.NoError(t, addHostKey(srv, keyPath))
	require.Len(t, srv.HostSigners, 1)

	assert.Equal(t, gossh.KeyAlgoED25519, srv.HostSigners[0].PublicKey().Type())
}

func TestInitKeys(t *testing.T) {
	tempDir := t.TempDir()

	keyTypes := []string{"rsa", "ecdsa", "ed25519"}
	for _, keyType := range keyTypes {
		privKeyPath := filepath.Join(tempDir, "gitea."+keyType)
		pubKeyPath := filepath.Join(tempDir, "gitea."+keyType+".pub")
		assert.NoFileExists(t, privKeyPath)
		assert.NoFileExists(t, pubKeyPath)
	}

	// Test basic creation
	keyFiles, err := InitDefaultHostKeys(tempDir)
	require.NoError(t, err)
	assert.Len(t, keyFiles, len(keyTypes))

	// Record file contents so regeneration can be detected
	content := map[string][]byte{}
	for _, keyType := range keyTypes {
		privKeyPath := filepath.Join(tempDir, "gitea."+keyType)
		pubKeyPath := filepath.Join(tempDir, "gitea."+keyType+".pub")
		data, err := os.ReadFile(privKeyPath)
		require.NoError(t, err)
		content[privKeyPath] = data

		data, err = os.ReadFile(pubKeyPath)
		require.NoError(t, err)
		content[pubKeyPath] = data
	}

	// Test recreation on missing private key and noop for missing pub key
	require.NoError(t, os.Remove(filepath.Join(tempDir, "gitea.ecdsa.pub")))
	require.NoError(t, os.Remove(filepath.Join(tempDir, "gitea.ed25519")))

	keyFiles, err = InitDefaultHostKeys(tempDir)
	require.NoError(t, err)
	assert.Len(t, keyFiles, len(keyTypes))

	for _, keyType := range keyTypes {
		privKeyPath := filepath.Join(tempDir, "gitea."+keyType)
		pubKeyPath := filepath.Join(tempDir, "gitea."+keyType+".pub")

		dataPriv, err := os.ReadFile(privKeyPath)
		require.NoError(t, err)

		switch keyType {
		case "rsa":
			// No modification to RSA key
			dataPub, err := os.ReadFile(pubKeyPath)
			require.NoError(t, err)
			assert.Equal(t, content[privKeyPath], dataPriv)
			assert.Equal(t, content[pubKeyPath], dataPub)
		case "ecdsa":
			// ECDSA public key should be missing, private unchanged
			assert.Equal(t, content[privKeyPath], dataPriv)
			assert.NoFileExists(t, pubKeyPath)
		case "ed25519":
			// ed25519 private key was removed, so both keys regenerated
			dataPub, err := os.ReadFile(pubKeyPath)
			require.NoError(t, err)
			assert.NotEqual(t, content[privKeyPath], dataPriv)
			assert.NotEqual(t, content[pubKeyPath], dataPub)
		}
	}
}
