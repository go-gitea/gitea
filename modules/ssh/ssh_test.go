// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ssh

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
)

func TestGenKeyPair(t *testing.T) {
	testCases := []struct {
		keyType      string
		expectedType any
	}{
		{
			keyType:      "rsa",
			expectedType: &rsa.PrivateKey{},
		},
		{
			keyType:      "ed25519",
			expectedType: &ed25519.PrivateKey{},
		},
		{
			keyType:      "ecdsa",
			expectedType: &ecdsa.PrivateKey{},
		},
	}
	for _, tC := range testCases {
		t.Run("Generate "+filepath.Ext(tC.keyType), func(t *testing.T) {
			path := t.TempDir() + "gitea." + tC.keyType
			require.NoError(t, GenKeyPair(path, tC.keyType))

			file, err := os.Open(path)
			require.NoError(t, err)

			bytes, err := io.ReadAll(file)
			require.NoError(t, err)

			privateKey, err := gossh.ParseRawPrivateKey(bytes)
			require.NoError(t, err)
			assert.IsType(t, tC.expectedType, privateKey)
		})
	}
	t.Run("Generate unknown keytype", func(t *testing.T) {
		path := t.TempDir() + "gitea.badkey"

		err := GenKeyPair(path, "badkey")
		require.Error(t, err)
	})
}

func TestInitKeys(t *testing.T) {
	tempDir := t.TempDir()

	keytypes := []string{"rsa", "ecdsa", "ed25519"}
	for _, keytype := range keytypes {
		privKeyPath := filepath.Join(tempDir, "gitea."+keytype)
		pubKeyPath := filepath.Join(tempDir, "gitea."+keytype+".pub")
		assert.NoFileExists(t, privKeyPath)
		assert.NoFileExists(t, pubKeyPath)
	}

	// Test basic creation
	err := initDefaultKeys(tempDir)
	require.NoError(t, err)

	metadata := map[string]os.FileInfo{}
	for _, keytype := range keytypes {
		privKeyPath := filepath.Join(tempDir, "gitea."+keytype)
		pubKeyPath := filepath.Join(tempDir, "gitea."+keytype+".pub")
		assert.FileExists(t, privKeyPath)
		assert.FileExists(t, pubKeyPath)

		info, err := os.Stat(privKeyPath)
		require.NoError(t, err)
		metadata[privKeyPath] = info

		info, err = os.Stat(pubKeyPath)
		require.NoError(t, err)
		metadata[pubKeyPath] = info
	}

	// Test recreation on missing public or private key
	require.NoError(t, os.Remove(filepath.Join(tempDir, "gitea.ecdsa.pub")))
	require.NoError(t, os.Remove(filepath.Join(tempDir, "gitea.ed25519")))

	err = initDefaultKeys(tempDir)
	require.NoError(t, err)

	for _, keytype := range keytypes {
		privKeyPath := filepath.Join(tempDir, "gitea."+keytype)
		pubKeyPath := filepath.Join(tempDir, "gitea."+keytype+".pub")
		assert.FileExists(t, privKeyPath)
		assert.FileExists(t, pubKeyPath)

		infoPriv, err := os.Stat(privKeyPath)
		require.NoError(t, err)
		infoPub, err := os.Stat(pubKeyPath)
		require.NoError(t, err)
		if keytype == "rsa" {
			assert.Equal(t, metadata[privKeyPath], infoPriv)
			assert.Equal(t, metadata[pubKeyPath], infoPub)
		} else {
			assert.NotEqual(t, metadata[privKeyPath], infoPriv)
			assert.NotEqual(t, metadata[pubKeyPath], infoPub)
		}
	}
}
