// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package lfs

import (
	"strings"
	"testing"

	perm_model "code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
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
	repoFork := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 11})

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

	t.Run("maintainer edits", func(t *testing.T) {
		maintainer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 12})
		ctx2, _ := contexttest.MockContext(t, "/")
		ctx2.Doer = maintainer
		if ctx2.Data == nil {
			ctx2.Data = make(map[string]any)
		}
		ctx2.Data[lfsRefContextKey] = "branch2"
		perm, err := access_model.GetUserRepoPermission(ctx2, repoFork, maintainer)
		require.NoError(t, err)
		require.False(t, perm.CanWrite(unit.TypeCode))
		assert.True(t, authenticate(ctx2, repoFork, "", false, true))
	})
}
