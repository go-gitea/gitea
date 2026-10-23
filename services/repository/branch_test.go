// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"

	"codenest.dev/models/db"
	git_model "codenest.dev/models/git"
	repo_model "codenest.dev/models/repo"
	"codenest.dev/models/unit"
	"codenest.dev/models/unittest"
	user_model "codenest.dev/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCanDeleteBranchDefaultTargets(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	ctx := t.Context()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	rule := &git_model.ProtectedBranch{
		RepoID: repo.ID, RuleName: "*",
		ProtectedBranchConfig: git_model.ProtectedBranchConfig{CanPush: true, CanDelete: true},
	}
	require.NoError(t, db.Insert(ctx, rule))
	assert.NoError(t, CanDeleteBranch(ctx, repo, "branch2", owner))
	assert.ErrorIs(t, CanDeleteBranch(ctx, repo, repo.DefaultBranch, owner), ErrBranchIsDefault)

	pullRequestUnit, err := repo.GetUnit(ctx, unit.TypePullRequests)
	assert.NoError(t, err)
	pullRequestConfig := pullRequestUnit.PullRequestsConfig()
	pullRequestConfig.DefaultTargetBranch = "branch2"
	pullRequestUnit.Config = pullRequestConfig
	assert.NoError(t, repo_model.UpdateRepoUnitConfig(ctx, pullRequestUnit))
	repo.Units = nil

	assert.ErrorIs(t, CanDeleteBranch(ctx, repo, "branch2", owner), ErrBranchIsDefault)
}
