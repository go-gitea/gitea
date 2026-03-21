// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package lfs

import (
	"strings"
	"testing"

	perm_model "code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/services/contexttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func TestAuthenticate(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	token2, _ := GetLFSAuthTokenWithBearer(AuthTokenOptions{Op: "download", UserID: 2, RepoID: 1})
	_, token2, _ = strings.Cut(token2, " ")
	ctx, _ := contexttest.MockContext(t, "/")

	t.Run("handleLFSToken", func(t *testing.T) {
		u, err := handleLFSToken(ctx, "", repo1, perm_model.AccessModeRead)
		require.Error(t, err)
		assert.Nil(t, u)

		u, err = handleLFSToken(ctx, "invalid", repo1, perm_model.AccessModeRead)
		require.Error(t, err)
		assert.Nil(t, u)

		u, err = handleLFSToken(ctx, token2, repo1, perm_model.AccessModeRead)
		require.NoError(t, err)
		assert.EqualValues(t, 2, u.ID)
	})

	t.Run("authenticate", func(t *testing.T) {
		const prefixBearer = "Bearer "
		assert.False(t, authenticate(ctx, repo1, "", true, false))
		assert.False(t, authenticate(ctx, repo1, prefixBearer+"invalid", true, false))
		assert.True(t, authenticate(ctx, repo1, prefixBearer+token2, true, false))
	})
}
