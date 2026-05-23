// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/builder"
)

func TestPullRequestList(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	t.Run("LoadAttributes", testPullRequestListLoadAttributes)
	t.Run("LoadReviewCommentsCounts", testPullRequestListLoadReviewCommentsCounts)
	t.Run("LoadReviews", testPullRequestListLoadReviews)
	t.Run("CanMaintainerWriteToBranch", testCanMaintainerWriteToBranch)
}

func testPullRequestListLoadAttributes(t *testing.T) {
	prs := issues_model.PullRequestList{
		unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 1}),
		unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2}),
	}
	assert.NoError(t, prs.LoadAttributes(t.Context()))
	for _, pr := range prs {
		assert.NotNil(t, pr.Issue)
		assert.Equal(t, pr.IssueID, pr.Issue.ID)
	}

	assert.NoError(t, issues_model.PullRequestList([]*issues_model.PullRequest{}).LoadAttributes(t.Context()))
}

func testPullRequestListLoadReviewCommentsCounts(t *testing.T) {
	prs := issues_model.PullRequestList{
		unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 1}),
		unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2}),
	}
	reviewComments, err := prs.LoadReviewCommentsCounts(t.Context())
	assert.NoError(t, err)
	assert.Len(t, reviewComments, 2)
	for _, pr := range prs {
		assert.Equal(t, 1, reviewComments[pr.IssueID])
	}
}

func testPullRequestListLoadReviews(t *testing.T) {
	prs := issues_model.PullRequestList{
		unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 1}),
		unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2}),
	}
	reviewList, err := prs.LoadReviews(t.Context())
	assert.NoError(t, err)
	// 1, 7, 8, 9, 10, 22
	assert.Len(t, reviewList, 6)
	assert.EqualValues(t, 1, reviewList[0].ID)
	assert.EqualValues(t, 7, reviewList[1].ID)
	assert.EqualValues(t, 8, reviewList[2].ID)
	assert.EqualValues(t, 9, reviewList[3].ID)
	assert.EqualValues(t, 10, reviewList[4].ID)
	assert.EqualValues(t, 22, reviewList[5].ID)
}

func testCanMaintainerWriteToBranch(t *testing.T) {
	ctx := t.Context()
	baseRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})
	headRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 11})

	_ = baseRepo.LoadOwner(ctx)
	_ = headRepo.LoadOwner(ctx)
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	// a PR from header's owner
	headOwnerPR := &issues_model.PullRequest{
		Issue: &issues_model.Issue{
			RepoID:   baseRepo.ID,
			PosterID: headRepo.OwnerID,
		},
		HeadRepoID: headRepo.ID,
		BaseRepoID: baseRepo.ID,
		HeadBranch: "pr-from-head-owner",
		BaseBranch: "master",
	}
	require.NoError(t, issues_model.NewPullRequest(ctx, baseRepo, headOwnerPR.Issue, nil, nil, headOwnerPR))

	// a PR from a user, they might have or not have "write" permission in the target repo
	anyUserPR := &issues_model.PullRequest{
		Issue: &issues_model.Issue{
			RepoID:   baseRepo.ID,
			PosterID: user.ID,
		},
		HeadRepoID: headRepo.ID,
		BaseRepoID: baseRepo.ID,
		HeadBranch: "pr-from-head-user",
		BaseBranch: "master",
	}
	require.NoError(t, issues_model.NewPullRequest(ctx, baseRepo, anyUserPR.Issue, nil, nil, anyUserPR))

	doerCanWrite := func(doer *user_model.User, pr *issues_model.PullRequest) bool {
		headPerm, _ := access.GetIndividualUserRepoPermission(ctx, headRepo, doer)
		return issues_model.CanMaintainerWriteToBranch(ctx, headPerm, pr.HeadBranch, doer)
	}

	t.Run("NoAllowMaintainerEdit", func(t *testing.T) {
		assert.True(t, doerCanWrite(headRepo.Owner, headOwnerPR))
		assert.False(t, doerCanWrite(baseRepo.Owner, headOwnerPR))
		assert.False(t, doerCanWrite(baseRepo.Owner, anyUserPR))
		assert.False(t, doerCanWrite(user, anyUserPR))
	})

	t.Run("WithAllowMaintainerEdit-HeadPosterReader", func(t *testing.T) {
		_, err := db.GetEngine(ctx).Where(builder.In("id", []int64{headOwnerPR.ID, anyUserPR.ID})).
			Cols("allow_maintainer_edit").
			Update(&issues_model.PullRequest{AllowMaintainerEdit: true})
		require.NoError(t, err)
		assert.True(t, doerCanWrite(baseRepo.Owner, headOwnerPR))
		assert.False(t, doerCanWrite(baseRepo.Owner, anyUserPR)) // poster doesn't have write permission, so maintainer can't write either
	})

	t.Run("WithAllowMaintainerEdit-HeadPosterWriter", func(t *testing.T) {
		_, err := db.GetEngine(ctx).Where(builder.In("id", []int64{headOwnerPR.ID, anyUserPR.ID})).
			Cols("allow_maintainer_edit").
			Update(&issues_model.PullRequest{AllowMaintainerEdit: true})
		require.NoError(t, err)
		err = db.Insert(ctx, &repo_model.Collaboration{RepoID: headRepo.ID, UserID: user.ID, Mode: perm.AccessModeWrite})
		require.NoError(t, err)
		err = db.Insert(ctx, &access.Access{RepoID: headRepo.ID, UserID: user.ID, Mode: perm.AccessModeWrite})
		require.NoError(t, err)
		assert.True(t, doerCanWrite(baseRepo.Owner, headOwnerPR))
		assert.True(t, doerCanWrite(baseRepo.Owner, anyUserPR)) // now the poster has the write permission
	})
}
