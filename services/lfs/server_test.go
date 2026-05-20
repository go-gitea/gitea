// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package lfs

import (
	"strings"
	"testing"

	"code.gitea.io/gitea/models/db"
	perm_model "code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
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
	ctx, _ := contexttest.MockContext(t, "/")

	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	getUserToken := func(op string, userID int64, repo *repo_model.Repository) string {
		s, _ := GetLFSAuthTokenWithBearer(AuthTokenOptions{Op: op, UserID: userID, RepoID: repo.ID})
		_, token, _ := strings.Cut(s, " ")
		return token
	}

	t.Run("handleLFSToken", func(t *testing.T) {
		u, err := handleLFSToken(ctx, "", repo1, perm_model.AccessModeRead)
		require.Error(t, err)
		assert.Nil(t, u)

		u, err = handleLFSToken(ctx, "invalid", repo1, perm_model.AccessModeRead)
		require.Error(t, err)
		assert.Nil(t, u)

		u, err = handleLFSToken(ctx, getUserToken("download", 2, repo1), repo1, perm_model.AccessModeRead)
		require.NoError(t, err)
		assert.EqualValues(t, 2, u.ID)
	})

	t.Run("authenticate", func(t *testing.T) {
		const prefixBearer = "Bearer "
		token := getUserToken("download", 2, repo1)
		assert.False(t, authenticate(ctx, repo1, "", true, false))
		assert.False(t, authenticate(ctx, repo1, prefixBearer+"invalid", true, false))
		assert.True(t, authenticate(ctx, repo1, prefixBearer+token, true, false))
	})

	handleLFSTokenTestPerm := func(op string, userID int64, repo *repo_model.Repository, accessMode perm_model.AccessMode) error {
		token := getUserToken(op, userID, repo)
		u, err := handleLFSToken(ctx, token, repo, accessMode)
		if err == nil {
			assert.Equal(t, userID, u.ID)
		}
		return err
	}

	t.Run("handleLFSToken blocks prohibited users", func(t *testing.T) {
		user37 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 37})

		// prohibited user
		assert.True(t, user37.ProhibitLogin)
		err := handleLFSTokenTestPerm("download", 37, repo1, perm_model.AccessModeRead)
		assert.ErrorContains(t, err, "not allowed to access any repository")

		// normal user
		_, _ = db.GetEngine(t.Context()).ID(37).Cols("prohibit_login").Update(&user_model.User{ProhibitLogin: false})
		err = handleLFSTokenTestPerm("download", 37, repo1, perm_model.AccessModeRead)
		assert.NoError(t, err)

		// inactive user
		_, _ = db.GetEngine(t.Context()).ID(37).Cols("is_active").Update(&user_model.User{IsActive: false})
		err = handleLFSTokenTestPerm("download", 37, repo1, perm_model.AccessModeRead)
		assert.ErrorContains(t, err, "not allowed to access any repository")
	})

	t.Run("handleLFSToken blocks users without repo access", func(t *testing.T) {
		repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
		err := handleLFSTokenTestPerm("download", 10, repo2, perm_model.AccessModeRead)
		assert.ErrorContains(t, err, "no permission to access the repository")
	})

	t.Run("handleLFSToken requires write access for uploads", func(t *testing.T) {
		err := handleLFSTokenTestPerm("download", 10, repo1, perm_model.AccessModeRead)
		assert.NoError(t, err)
		err = handleLFSTokenTestPerm("upload", 10, repo1, perm_model.AccessModeWrite)
		assert.ErrorContains(t, err, "no permission to access the repository")
	})

	t.Run("handleLFSToken allows writes for authorized users", func(t *testing.T) {
		err := handleLFSTokenTestPerm("upload", 2, repo1, perm_model.AccessModeWrite)
		assert.NoError(t, err)
	})
}
