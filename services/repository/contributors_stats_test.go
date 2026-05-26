// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	contribution_model "code.gitea.io/gitea/models/repo/contribution"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func TestProcessContributorStatsUpdateWithoutMetaRequestsRebuild(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	_, err := db.GetEngine(t.Context()).ID(repo.ID).Delete(new(contribution_model.ContributorMeta))
	assert.NoError(t, err)

	assert.NoError(t, processContributorStatsUpdate(t.Context(), &ContributorStatsUpdateOptions{RepoID: repo.ID}))

	meta, has, err := contribution_model.GetRepoContributorMeta(t.Context(), repo.ID)
	assert.NoError(t, err)
	if assert.True(t, has) {
		assert.True(t, meta.Dirty)
		assert.Empty(t, meta.LastProcessedCommitID)
	}
}

func TestProcessContributorStatsUpdateUsesDefaultBranchCommitFromDB(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	gitRepo, err := gitrepo.OpenRepository(t.Context(), repo)
	assert.NoError(t, err)
	if !assert.NotNil(t, gitRepo) {
		return
	}
	defer gitRepo.Close()

	defaultBranch, err := git_model.GetBranch(t.Context(), repo.ID, repo.DefaultBranch)
	if git_model.IsErrBranchNotExist(err) {
		headCommit, getErr := gitRepo.GetCommit(repo.DefaultBranch)
		assert.NoError(t, getErr)
		if !assert.NotNil(t, headCommit) {
			return
		}
		assert.NoError(t, git_model.AddBranches(t.Context(), []*git_model.Branch{{
			RepoID:   repo.ID,
			Name:     repo.DefaultBranch,
			CommitID: headCommit.ID.String(),
		}}))
		defaultBranch, err = git_model.GetBranch(t.Context(), repo.ID, repo.DefaultBranch)
	}
	assert.NoError(t, err)
	if !assert.NotNil(t, defaultBranch) {
		return
	}

	headCommit, err := gitRepo.GetCommit(defaultBranch.CommitID)
	assert.NoError(t, err)
	if !assert.NotNil(t, headCommit) {
		return
	}

	commits, err := headCommit.CommitsBeforeLimit(2)
	assert.NoError(t, err)
	if !assert.Len(t, commits, 2) {
		return
	}

	assert.NoError(t, contribution_model.DeleteRepoContributorDailyStats(t.Context(), repo.ID))
	_, err = db.GetEngine(t.Context()).ID(repo.ID).Delete(new(contribution_model.ContributorMeta))
	assert.NoError(t, err)

	meta, err := contribution_model.EnsureRepoContributorMeta(t.Context(), repo.ID)
	assert.NoError(t, err)
	meta.LastProcessedCommitID = commits[1].ID.String()
	meta.Dirty = false
	meta.UpdatedUnix = timeutil.TimeStampNow()
	assert.NoError(t, contribution_model.UpdateRepoContributorMeta(t.Context(), meta, "last_processed_commit_id", "dirty", "updated_unix"))

	assert.NoError(t, processContributorStatsUpdate(t.Context(), &ContributorStatsUpdateOptions{RepoID: repo.ID}))

	var has bool
	meta, has, err = contribution_model.GetRepoContributorMeta(t.Context(), repo.ID)
	assert.NoError(t, err)
	if assert.True(t, has) {
		assert.Equal(t, defaultBranch.CommitID, meta.LastProcessedCommitID)
		assert.False(t, meta.Dirty)
	}
}
