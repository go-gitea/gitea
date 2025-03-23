// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ssh_test

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"io"
	"os"
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/modules/ssh"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
)

func TestGenKeyPair(t *testing.T) {
	testCases := []struct {
		keyPath      string
		expectedType any
	}{
		{
			keyPath:      "/gitea.rsa",
			expectedType: &rsa.PrivateKey{},
		},
		{
			keyPath:      "/gitea.ed25519",
			expectedType: &ed25519.PrivateKey{},
		},
		{
			keyPath:      "/gitea.ecdsa",
			expectedType: &ecdsa.PrivateKey{},
		},
	}
	for _, tC := range testCases {
		t.Run("Generate "+filepath.Ext(tC.keyPath), func(t *testing.T) {
			path := t.TempDir() + tC.keyPath
			require.NoError(t, ssh.GenKeyPair(path))

			file, err := os.Open(path)
			require.NoError(t, err)

			bytes, err := io.ReadAll(file)
			require.NoError(t, err)

			privateKey, err := gossh.ParseRawPrivateKey(bytes)
			require.NoError(t, err)
			assert.IsType(t, tC.expectedType, privateKey)
		})
	}
}
