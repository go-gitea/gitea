// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestUseOwnerSSHKey(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	orgOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	orgMember := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	org3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})
	const sshAddr = "ssh://git@example.com/repo.git"
	_, err := UseOwnerSSHKey(t.Context(), orgOwner, orgOwner, sshAddr)
	assert.ErrorIs(t, err, util.ErrUnprocessableContent)
	for _, keyOwnerID := range []int64{2, 3} {
		_, err := user_model.CreateSSHKeypair(t.Context(), keyOwnerID)
		require.NoError(t, err)
	}

	useOwnerKey, err := UseOwnerSSHKey(t.Context(), orgOwner, org3, "https://example.com/repo.git")
	require.NoError(t, err)
	assert.False(t, useOwnerKey)
	for _, owner := range []*user_model.User{orgOwner, org3} {
		useOwnerKey, err = UseOwnerSSHKey(t.Context(), orgOwner, owner, sshAddr)
		require.NoError(t, err)
		assert.True(t, useOwnerKey)
	}

	_, err = UseOwnerSSHKey(t.Context(), orgMember, org3, sshAddr)
	assert.ErrorIs(t, err, util.ErrPermissionDenied)
	_, err = UseOwnerSSHKey(t.Context(), orgMember, orgOwner, sshAddr)
	assert.ErrorIs(t, err, util.ErrPermissionDenied)
	publicOnly, err := user_model.GetDoerPermissionUser(t.Context(), 2, user_model.NewTokenExtDoerData(user_model.CredentialAccessToken, 1, "public-only,write:repository").EncodeToString())
	require.NoError(t, err)
	_, err = UseOwnerSSHKey(t.Context(), publicOnly, orgOwner, sshAddr)
	assert.ErrorIs(t, err, util.ErrPermissionDenied)
}

func TestManagedSSHEnv(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	org3Repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 5})
	const sshAddr = "git@example.com:repo.git"

	envs, cleanup, err := ManagedSSHEnv(t.Context(), org3Repo, true, "https://example.com/repo.git")
	require.NoError(t, err)
	cleanup()
	assert.Nil(t, envs)
	_, _, err = ManagedSSHEnv(t.Context(), org3Repo, false, sshAddr)
	assert.ErrorIs(t, err, util.ErrPermissionDenied)
	_, _, err = ManagedSSHEnv(t.Context(), org3Repo, true, sshAddr)
	assert.ErrorIs(t, err, util.ErrNotExist)

	ownerKey, err := user_model.CreateSSHKeypair(t.Context(), 3)
	require.NoError(t, err)
	envs, cleanup, err = ManagedSSHEnv(t.Context(), org3Repo, true, sshAddr)
	require.NoError(t, err)
	sshCommand, ok := strings.CutPrefix(envs[len(envs)-1], "GIT_SSH_COMMAND=")
	require.True(t, ok)
	keyFiles, _ := filepath.Glob(filepath.Join(setting.AppDataTempDir("ssh").JoinPath(), "key-*", "id"))
	require.Len(t, keyFiles, 1)
	knownHosts := filepath.Join(setting.Git.HomePath, ".ssh", "known_hosts")
	for _, option := range []string{"-o BatchMode=yes", "-o IdentitiesOnly=yes", "-i " + keyFiles[0], "-o StrictHostKeyChecking=accept-new", `-o "UserKnownHostsFile=\"` + knownHosts + `\""`} {
		assert.Contains(t, sshCommand, option)
	}
	privateKey, err := os.ReadFile(keyFiles[0])
	require.NoError(t, err)
	signer, err := ssh.ParsePrivateKey(privateKey)
	require.NoError(t, err)
	assert.Equal(t, ownerKey.Fingerprint, ssh.FingerprintSHA256(signer.PublicKey()))
	cleanup()
	assert.NoFileExists(t, keyFiles[0])

	defer test.MockVariableValue(&setting.Migrations.SSHHostKeyChecking, "no")()
	assert.Contains(t, managedSSHCommand("id", knownHosts), `-o StrictHostKeyChecking=no -o "UserKnownHostsFile=\"`+os.DevNull+`\""`)

	defer test.MockVariableValue(&setting.Migrations.SSHCommand, "/opt/ssh")()
	assert.True(t, strings.HasPrefix(managedSSHCommand("id", knownHosts), util.ShellEscape("/opt/ssh")+" "))
}
