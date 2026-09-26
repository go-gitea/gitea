// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ssh

import (
	"os"
	"strings"
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	_ "gitea.dev/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, &unittest.TestOptions{FixtureFiles: []string{ /* keypairs are created by the tests themselves */ }})
}

func TestIsSSHURL(t *testing.T) {
	cases := []struct {
		remote string
		isSSH  bool
	}{
		{"ssh://git@github.com/user/repo.git", true},
		{"ssh://git@example.com:2222/user/repo.git", true},
		{"git@github.com:user/repo.git", true},
		{"https://github.com/user/repo.git", false},
		{"http://github.com/user/repo.git", false},
		{"git://github.com/user/repo.git", false},
		{"/srv/git/repo.git", false},
		{"", false},
	}
	for _, c := range cases {
		assert.Equal(t, c.isSSH, IsSSHURL(c.remote), "remote %q", c.remote)
	}
}

func TestSetupManagedSSHAgentNonSSHURL(t *testing.T) {
	// a non-SSH remote must short-circuit before any keypair lookup
	sshEnvs, cleanup, err := SetupManagedSSHAgent(t.Context(), &repo_model.Repository{OwnerID: 1}, "https://github.com/user/repo.git", 0)
	require.NoError(t, err)
	assert.Empty(t, sshEnvs)
	require.NotNil(t, cleanup)
	cleanup()
}

func TestSetupManagedSSHAgent(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	defer withTestAppDataPath(t)()

	t.Run("uses the repository owner's key", func(t *testing.T) {
		sshEnvs, cleanup, err := SetupManagedSSHAgent(t.Context(), &repo_model.Repository{OwnerID: 2}, "ssh://git@github.com/user/repo.git", 0)
		require.NoError(t, err)
		defer cleanup()

		require.Len(t, sshEnvs, 2)
		authSock := envValue(t, sshEnvs, "SSH_AUTH_SOCK=")
		assert.NotEmpty(t, authSock)

		sshCommand := envValue(t, sshEnvs, "GIT_SSH_COMMAND=")
		assert.Contains(t, sshCommand, "-o BatchMode=yes")
		identityFile := identityFileFrom(t, sshCommand)
		content, err := os.ReadFile(identityFile)
		require.NoError(t, err)

		keypair, err := user_model.GetSSHKeypairByOwner(t.Context(), 2)
		require.NoError(t, err)
		assert.Equal(t, keypair.PublicKey, string(content))

		cleanup()
		assert.NoFileExists(t, identityFile)
	})

	t.Run("sshKeyOwnerID overrides the repository owner", func(t *testing.T) {
		sshEnvs, cleanup, err := SetupManagedSSHAgent(t.Context(), &repo_model.Repository{OwnerID: 2}, "ssh://git@github.com/user/repo.git", 4)
		require.NoError(t, err)
		defer cleanup()

		identityFile := identityFileFrom(t, envValue(t, sshEnvs, "GIT_SSH_COMMAND="))
		content, err := os.ReadFile(identityFile)
		require.NoError(t, err)

		keypair, err := user_model.GetSSHKeypairByOwner(t.Context(), 4)
		require.NoError(t, err)
		assert.Equal(t, keypair.PublicKey, string(content))
	})
}

func TestWriteManagedPublicKey(t *testing.T) {
	const publicKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAItest gitea"

	path, cleanup, err := writeManagedPublicKey(publicKey)
	require.NoError(t, err)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, publicKey, string(content))

	cleanup()
	assert.NoFileExists(t, path)
}

func envValue(t *testing.T, envs []string, prefix string) string {
	t.Helper()
	for _, env := range envs {
		if after, ok := strings.CutPrefix(env, prefix); ok {
			return after
		}
	}
	require.Failf(t, "missing env", "no %q in %v", prefix, envs)
	return ""
}

func identityFileFrom(t *testing.T, sshCommand string) string {
	t.Helper()
	_, after, ok := strings.Cut(sshCommand, "-o IdentitiesOnly=yes -i ")
	require.True(t, ok, "no identity file in %q", sshCommand)
	return strings.Trim(after, `'"`)
}
