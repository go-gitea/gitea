// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddHTTPSDeployKey(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	key, token, err := AddHTTPSDeployKey(t.Context(), 1, "ci-readonly", true)
	require.NoError(t, err)
	require.NotNil(t, key)

	assert.Equal(t, int64(1), key.RepoID)
	assert.Equal(t, "ci-readonly", key.Name)
	assert.True(t, key.IsReadOnly())
	assert.Len(t, token, 40, "token should be a 40-char hex string")
	for _, r := range token {
		ok := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')
		assert.True(t, ok, "token contains non-hex char %q", r)
	}

	got, err := GetHTTPSDeployKeyByID(t.Context(), key.ID)
	require.NoError(t, err)
	assert.Equal(t, key.ID, got.ID)
	assert.Equal(t, key.TokenHash, got.TokenHash)
	assert.Empty(t, got.Token, "plaintext token must not be persisted")
}

func TestAddHTTPSDeployKey_NameUnique(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	_, _, err := AddHTTPSDeployKey(t.Context(), 1, "dup", false)
	require.NoError(t, err)

	_, _, err = AddHTTPSDeployKey(t.Context(), 1, "dup", false)
	require.Error(t, err)
	assert.True(t, IsErrHTTPSDeployKeyNameAlreadyUsed(err),
		"expected ErrHTTPSDeployKeyNameAlreadyUsed, got %T: %v", err, err)

	// Same name on a different repo is fine.
	_, _, err = AddHTTPSDeployKey(t.Context(), 2, "dup", false)
	require.NoError(t, err)
}

func TestListHTTPSDeployKeys(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	_, _, err := AddHTTPSDeployKey(t.Context(), 1, "a", true)
	require.NoError(t, err)
	_, _, err = AddHTTPSDeployKey(t.Context(), 1, "b", false)
	require.NoError(t, err)
	_, _, err = AddHTTPSDeployKey(t.Context(), 2, "c", true)
	require.NoError(t, err)

	keys, err := db.Find[HTTPSDeployKey](t.Context(),
		ListHTTPSDeployKeysOptions{RepoID: 1})
	require.NoError(t, err)
	assert.Len(t, keys, 2)

	keys, err = db.Find[HTTPSDeployKey](t.Context(),
		ListHTTPSDeployKeysOptions{RepoID: 2})
	require.NoError(t, err)
	assert.Len(t, keys, 1)
}

func TestDeleteHTTPSDeployKey(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	key, _, err := AddHTTPSDeployKey(t.Context(), 1, "to-delete", true)
	require.NoError(t, err)

	require.NoError(t, DeleteHTTPSDeployKey(t.Context(), 1, key.ID))

	_, err = GetHTTPSDeployKeyByID(t.Context(), key.ID)
	require.Error(t, err)
	assert.True(t, IsErrHTTPSDeployKeyNotExist(err))

	// Deleting a key that belongs to a different repo must fail cleanly.
	key, _, err = AddHTTPSDeployKey(t.Context(), 1, "stays", true)
	require.NoError(t, err)
	err = DeleteHTTPSDeployKey(t.Context(), 2, key.ID)
	require.Error(t, err)
	assert.True(t, IsErrHTTPSDeployKeyNotExist(err))
}

func TestVerifyHTTPSDeployToken(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	key, token, err := AddHTTPSDeployKey(t.Context(), 1, "verify", false)
	require.NoError(t, err)

	got, err := VerifyHTTPSDeployToken(t.Context(), token)
	require.NoError(t, err)
	assert.Equal(t, key.ID, got.ID)
	assert.Equal(t, key.RepoID, got.RepoID)

	_, err = VerifyHTTPSDeployToken(t.Context(), "0000000000000000000000000000000000000000")
	require.Error(t, err)
	assert.True(t, IsErrHTTPSDeployKeyNotExist(err))

	_, err = VerifyHTTPSDeployToken(t.Context(), "")
	require.Error(t, err)

	_, err = VerifyHTTPSDeployToken(t.Context(), "not-hex")
	require.Error(t, err)
}
