// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package deploykey

import (
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeployToken(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	key, err := AddDeployKeyToken(t.Context(), 1, "ci", false)
	require.NoError(t, err)
	assert.False(t, key.IsReadOnly())
	assert.Len(t, key.Token, len(DeployTokenPrefix)+deployTokenLength)

	got, err := VerifyDeployKeyToken(t.Context(), key.Token)
	require.NoError(t, err)
	assert.Equal(t, key.ID, got.ID)
	assert.Empty(t, got.Token, "the token itself is never stored")

	_, err = VerifyDeployKeyToken(t.Context(), "not-a-token")
	assert.True(t, IsErrDeployKeyNotExist(err))

	_, err = AddDeployKeyToken(t.Context(), 1, "ci", false)
	assert.True(t, IsErrDeployKeyNameAlreadyUsed(err))

	regenerated, err := RegenerateDeployKeyToken(t.Context(), 1, key.ID)
	require.NoError(t, err)
	assert.Equal(t, key.Name, regenerated.Name)
	assert.Equal(t, key.Mode, regenerated.Mode)
	assert.False(t, unittest.AssertExistsAndLoadBean(t, &DeployKey{ID: key.ID}).HasUsed(), "regenerating is not a use")

	_, err = VerifyDeployKeyToken(t.Context(), key.Token)
	assert.True(t, IsErrDeployKeyNotExist(err), "the old token stops working")
	_, err = VerifyDeployKeyToken(t.Context(), regenerated.Token)
	require.NoError(t, err)

	// an SSH deploy-key has no token to regenerate
	sshKey := &DeployKey{RepoID: 1, KeyType: KeyTypeSSH, Name: "ssh"}
	require.NoError(t, db.Insert(t.Context(), sshKey))
	_, err = RegenerateDeployKeyToken(t.Context(), 1, sshKey.ID)
	assert.True(t, IsErrDeployKeyNotExist(err))
}
