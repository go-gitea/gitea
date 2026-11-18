// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"testing"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToAPIRepoConfidential(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	// 3 public + 2 confidential open issues; the stored counter keeps counting all of them
	addIssues := func(repo *repo_model.Repository, poster *user_model.User) {
		for index := range int64(5) {
			require.NoError(t, db.Insert(ctx, &issues_model.Issue{
				RepoID: repo.ID, Repo: repo, Index: 9000 + index, PosterID: poster.ID, Poster: poster,
				Title: "confidential count", IsConfidential: index >= 3,
			}))
		}
		repo.NumOpenIssues = 5
	}
	permissionOf := func(repo *repo_model.Repository, doer *user_model.User) access_model.Permission {
		p, err := access_model.GetDoerRepoPermission(ctx, repo, doer)
		require.NoError(t, err)
		return p
	}

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	addIssues(repo, owner)

	assert.Equal(t, 3, ToAPIRepo(ctx, repo, permissionOf(repo, nil), nil).OpenIssues)
	assert.Equal(t, 5, ToAPIRepo(ctx, repo, permissionOf(repo, owner), owner).OpenIssues)
	assert.Equal(t, 5, ToAPIRepo(ctx, repo, access_model.Permission{AccessMode: perm.AccessModeOwner}, owner).OpenIssues, "hard-coded permission")
	assert.Equal(t, 5, ToRepo(ctx, repo, permissionOf(repo, nil)).OpenIssues, "payloads keep the stored counter")

	t.Run("fork parent uses the permission on the parent", func(t *testing.T) {
		parent := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})
		parentOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 12})
		forkOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 13})
		addIssues(parent, parentOwner)
		fork := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 11})
		fork.NumOpenIssues = 5
		fork.BaseRepo = parent // keeps the in-memory counter
		forkPermission := access_model.Permission{AccessMode: perm.AccessModeOwner}

		apiFork := ToAPIRepo(ctx, fork, forkPermission, forkOwner)
		assert.Equal(t, 5, apiFork.OpenIssues)
		require.NotNil(t, apiFork.Parent)
		assert.Equal(t, 3, apiFork.Parent.OpenIssues)

		apiFork = ToAPIRepo(ctx, fork, forkPermission, parentOwner)
		assert.Equal(t, 5, apiFork.Parent.OpenIssues)
		assert.Equal(t, 5, ToRepo(ctx, fork, forkPermission).Parent.OpenIssues)
	})
}
