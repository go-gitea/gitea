// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
)

func TestStarRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	t.Run("IsStaring", func(t *testing.T) {
		assert.True(t, repo_model.IsStaring(t.Context(), 2, 4))
		assert.False(t, repo_model.IsStaring(t.Context(), 3, 4))
	})

	t.Run("Repository_GetStargazers", func(t *testing.T) {
		// repo with stargazers
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
		gazers, err := repo_model.GetStargazers(t.Context(), repo, db.ListOptions{Page: 0})
		assert.NoError(t, err)
		if assert.Len(t, gazers, 1) {
			assert.Equal(t, int64(2), gazers[0].ID)
		}
	})

	t.Run("Repository_GetStargazers2", func(t *testing.T) {
		// repo with stargazers
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})
		gazers, err := repo_model.GetStargazers(t.Context(), repo, db.ListOptions{Page: 0})
		assert.NoError(t, err)
		assert.Empty(t, gazers)
	})

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	unittest.AssertNotExistsBean(t, &repo_model.Star{UID: user.ID, RepoID: repo.ID})
	assert.NoError(t, repo_model.StarRepo(t.Context(), user, repo, true))
	unittest.AssertExistsAndLoadBean(t, &repo_model.Star{UID: user.ID, RepoID: repo.ID})
	assert.NoError(t, repo_model.StarRepo(t.Context(), user, repo, true))
	unittest.AssertExistsAndLoadBean(t, &repo_model.Star{UID: user.ID, RepoID: repo.ID})
	assert.NoError(t, repo_model.StarRepo(t.Context(), user, repo, false))
	unittest.AssertNotExistsBean(t, &repo_model.Star{UID: user.ID, RepoID: repo.ID})

	t.Run("ClearRepoStars", func(t *testing.T) {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

		unittest.AssertNotExistsBean(t, &repo_model.Star{UID: user.ID, RepoID: repo.ID})
		assert.NoError(t, repo_model.StarRepo(t.Context(), user, repo, true))
		unittest.AssertExistsAndLoadBean(t, &repo_model.Star{UID: user.ID, RepoID: repo.ID})
		assert.NoError(t, repo_model.StarRepo(t.Context(), user, repo, false))
		unittest.AssertNotExistsBean(t, &repo_model.Star{UID: user.ID, RepoID: repo.ID})
		assert.NoError(t, repo_model.ClearRepoStars(t.Context(), repo.ID))
		unittest.AssertNotExistsBean(t, &repo_model.Star{UID: user.ID, RepoID: repo.ID})

		gazers, err := repo_model.GetStargazers(t.Context(), repo, db.ListOptions{Page: 0})
		assert.NoError(t, err)
		assert.Empty(t, gazers)
	})
}
