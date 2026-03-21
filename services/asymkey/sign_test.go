// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserHasPubkeys(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	test := func(t *testing.T, userID int64, expectedHasGPG, expectedHasSSH bool) {
		ctx := t.Context()
		hasGPG, err := userHasPubkeysGPG(ctx, userID)
		require.NoError(t, err)
		hasSSH, err := userHasPubkeysSSH(ctx, userID)
		require.NoError(t, err)
		hasPubkeys, err := userHasPubkeys(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, expectedHasGPG, hasGPG)
		assert.Equal(t, expectedHasSSH, hasSSH)
		assert.Equal(t, expectedHasGPG || expectedHasSSH, hasPubkeys)
	}

	t.Run("AllowUserWithGPGKey", func(t *testing.T) {
		test(t, 36, true, false) // has gpg
	})
	t.Run("AllowUserWithSSHKey", func(t *testing.T) {
		test(t, 2, false, true) // has ssh
	})
	t.Run("DenyUserWithNoKeys", func(t *testing.T) {
		test(t, 1, false, false) // no pubkey
	})
}
