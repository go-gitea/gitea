// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"testing"

	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeployToken(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	key, err := AddDeployToken(t.Context(), 1, "ci", false)
	require.NoError(t, err)
	assert.Equal(t, DeployKeyTypeToken, key.Type)
	assert.False(t, key.IsReadOnly())
	assert.Len(t, key.Token, len(DeployTokenPrefix)+deployTokenLength)

	got, err := VerifyDeployToken(t.Context(), key.Token)
	require.NoError(t, err)
	assert.Equal(t, key.ID, got.ID)
	assert.Empty(t, got.Token, "the token itself is never stored")

	_, err = VerifyDeployToken(t.Context(), "not-a-token")
	assert.True(t, IsErrDeployKeyNotExist(err))

	_, err = AddDeployToken(t.Context(), 1, "ci", false)
	assert.True(t, IsErrDeployKeyNameAlreadyUsed(err))
}
