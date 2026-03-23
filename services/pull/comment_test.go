// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/json"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreatePushPullCommentForcePushDeletesOldComments(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	assert.NoError(t, pr.LoadIssue(t.Context()))
	assert.NoError(t, pr.LoadBaseRepo(t.Context()))
	pusher := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	gitRepo, err := gitrepo.OpenRepository(t.Context(), pr.BaseRepo)
	require.NoError(t, err)
	defer gitRepo.Close()

	insertCommitComment := func(t *testing.T, content issues_model.PushActionContent) {
		contentJSON, _ := json.Marshal(content)
		_, err := issues_model.CreateComment(t.Context(), &issues_model.CreateCommentOptions{
			Type:    issues_model.CommentTypePullRequestPush,
			Doer:    pusher,
			Repo:    pr.BaseRepo,
			Issue:   pr.Issue,
			Content: string(contentJSON),
		})
		require.NoError(t, err)
	}

	assertCommitCommentCount := func(t *testing.T, expectedTotalCount, expectedForcePushCount int) {
		comments, err := issues_model.FindComments(t.Context(), &issues_model.FindCommentsOptions{
			IssueID: pr.IssueID,
			Type:    issues_model.CommentTypePullRequestPush,
		})
		assert.NoError(t, err)
		totalCount := len(comments)
		forcePushCount := 0
		for _, comment := range comments {
			pushData, err := comment.GetPushActionContent()
			require.NoError(t, err)
			if pushData.IsForcePush {
				forcePushCount++
			}
		}
		assert.Equal(t, expectedTotalCount, totalCount)
		assert.Equal(t, expectedForcePushCount, forcePushCount)
	}

	t.Run("base-branch-only", func(t *testing.T) {
		db.TruncateBeans(t.Context(), &issues_model.Comment{})
		insertCommitComment(t, issues_model.PushActionContent{})
		insertCommitComment(t, issues_model.PushActionContent{})
		assertCommitCommentCount(t, 2, 0)

		baseCommit, err := gitRepo.GetBranchCommit(pr.BaseBranch)
		assert.NoError(t, err)

		comment, err := CreatePushPullComment(t.Context(), pusher, pr, baseCommit.ID.String(), baseCommit.ID.String(), true)
		require.NoError(t, err)
		require.NotNil(t, comment)
		assertCommitCommentCount(t, 1, 1)
	})

	t.Run("force-push-ignores-missing-old-commit", func(t *testing.T) {
		headCommit, err := gitRepo.GetBranchCommit(pr.HeadBranch)
		require.NoError(t, err)

		comment, err := CreatePushPullComment(t.Context(), pusher, pr, "0000000000000000000000000000000000000000", headCommit.ID.String(), true)
		require.NoError(t, err)
		require.NotNil(t, comment)
		createdData, err := comment.GetPushActionContent()
		assert.True(t, createdData.IsForcePush)
		assert.NotEmpty(t, createdData.CommitIDs)
	})

	t.Run("head-vs-base-branch", func(t *testing.T) {
		db.TruncateBeans(t.Context(), &issues_model.Comment{})
		insertCommitComment(t, issues_model.PushActionContent{})
		insertCommitComment(t, issues_model.PushActionContent{})
		assertCommitCommentCount(t, 2, 0)

		headCommit, err := gitRepo.GetBranchCommit(pr.HeadBranch)
		require.NoError(t, err)
		baseCommit, err := gitRepo.GetBranchCommit(pr.BaseBranch)
		require.NoError(t, err)
		oldCommit := baseCommit

		_, err = CreatePushPullComment(t.Context(), pusher, pr, oldCommit.ID.String(), headCommit.ID.String(), true)
		require.NoError(t, err)
		// Two comments should exist now: one regular push comment and one force-push comment.
		assertCommitCommentCount(t, 2, 1)
	})
}
