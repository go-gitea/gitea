// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !windows

package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func dialTestAgent(t *testing.T, socketPath string) net.Conn {
	t.Helper()
	conn, err := net.Dial("unix", socketPath)
	require.NoError(t, err)
	return conn
}

func TestSSHAgentSocketPermissions(t *testing.T) {
	defer withTestAppDataPath(t)()

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	socketPath, cleanup, err := CreateTemporaryAgent(privateKey)
	require.NoError(t, err)

	info, err := os.Stat(socketPath)
	require.NoError(t, err)
	// the socket grants access to the private key, so it must not be reachable by other users
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	dirInfo, err := os.Stat(filepath.Dir(socketPath))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), dirInfo.Mode().Perm())

	cleanup()
	assert.NoDirExists(t, filepath.Dir(socketPath))
}
