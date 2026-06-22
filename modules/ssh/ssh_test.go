// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ssh

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"os"
	"path/filepath"
	"testing"

	"gitea.dev/modules/generate"

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

	metadata := map[string]os.FileInfo{}
	for _, keyType := range keyTypes {
		privKeyPath := filepath.Join(tempDir, "gitea."+keyType)
		pubKeyPath := filepath.Join(tempDir, "gitea."+keyType+".pub")
		info, err := os.Stat(privKeyPath)
		require.NoError(t, err)
		metadata[privKeyPath] = info

		info, err = os.Stat(pubKeyPath)
		require.NoError(t, err)
		metadata[pubKeyPath] = info
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

		infoPriv, err := os.Stat(privKeyPath)
		require.NoError(t, err)

		switch keyType {
		case "rsa":
			// No modification to RSA key
			infoPub, err := os.Stat(pubKeyPath)
			require.NoError(t, err)
			assert.Equal(t, metadata[privKeyPath], infoPriv)
			assert.Equal(t, metadata[pubKeyPath], infoPub)
		case "ecdsa":
			// ECDSA public key should be missing, private unchanged
			assert.Equal(t, metadata[privKeyPath], infoPriv)
			assert.NoFileExists(t, pubKeyPath)
		case "ed25519":
			// ed25519 private key was removed, so both keys regenerated
			infoPub, err := os.Stat(pubKeyPath)
			require.NoError(t, err)
			assert.NotEqual(t, metadata[privKeyPath], infoPriv)
			assert.NotEqual(t, metadata[pubKeyPath], infoPub)
		}
	}
}
