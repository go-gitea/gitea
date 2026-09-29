// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"

	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// withTestAppDataPath points the agent's temporary directory at the test's own,
// since setting.AppDataTempDir panics when AppDataPath is unset.
func withTestAppDataPath(t *testing.T) func() {
	t.Helper()
	orig := setting.AppDataPath
	setting.AppDataPath = t.TempDir()
	return func() { setting.AppDataPath = orig }
}

func TestCreateTemporaryAgent(t *testing.T) {
	defer withTestAppDataPath(t)()

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	socketPath, cleanup, err := CreateTemporaryAgent(privateKey)
	require.NoError(t, err)
	defer cleanup()
	require.NotEmpty(t, socketPath)

	conn := dialTestAgent(t, socketPath)
	defer conn.Close()
	client := agent.NewClient(conn)

	keys, err := client.List()
	require.NoError(t, err)
	require.Len(t, keys, 1)

	wantSigner, err := gossh.NewSignerFromKey(privateKey)
	require.NoError(t, err)
	assert.Equal(t, wantSigner.PublicKey().Marshal(), keys[0].Marshal())

	// the agent exists so git can authenticate with it, so verify it actually signs
	data := []byte("gitea managed ssh agent")
	sig, err := client.Sign(keys[0], data)
	require.NoError(t, err)
	assert.Equal(t, "ssh-ed25519", sig.Format)
	assert.True(t, ed25519.Verify(publicKey, data, sig.Blob))
}

func TestNewSSHAgentRejectsInvalidKey(t *testing.T) {
	defer withTestAppDataPath(t)()

	_, err := NewSSHAgent(ed25519.PrivateKey("too short"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid Ed25519 private key size")
}

func TestSSHAgentCloseIsIdempotent(t *testing.T) {
	defer withTestAppDataPath(t)()

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	sa, err := NewSSHAgent(privateKey)
	require.NoError(t, err)

	// mirror sync defers a cleanup that may run after an explicit close
	require.NoError(t, sa.Close())
	require.NoError(t, sa.Close())
}
