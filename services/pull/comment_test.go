// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/json"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreatePushPullCommentForcePushDeletesOldComments(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	pusher := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	require.NoError(t, pr.LoadIssue(t.Context()))
	require.NoError(t, pr.LoadBaseRepo(t.Context()))

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
		require.NoError(t, err)
		totalCount, forcePushCount := len(comments), 0
		for _, comment := range comments {
			pushData, err := comment.GetPushActionContent()
			require.NoError(t, err)
			if pushData.IsForcePush {
				forcePushCount++
			}
		}
		assert.Equal(t, expectedTotalCount, totalCount, "total comment count should match")
		assert.Equal(t, expectedForcePushCount, forcePushCount, "force push comment count should match")
	}

	t.Run("base-branch-only", func(t *testing.T) {
		require.NoError(t, db.TruncateBeans(t.Context(), &issues_model.Comment{}))
		insertCommitComment(t, issues_model.PushActionContent{})
		insertCommitComment(t, issues_model.PushActionContent{})
		assertCommitCommentCount(t, 2, 0)

		baseCommit, err := gitRepo.GetBranchCommit(pr.BaseBranch)
		assert.NoError(t, err)

		// force push, the old push comments should be deleted, and one new force-push comment should be created.
		// the pushed branch is the same as base branch, so no commit between old and new commit, no regular push comment
		comment, _, err := CreatePushPullComment(t.Context(), pusher, pr, baseCommit.ID.String(), baseCommit.ID.String(), true)
		require.NoError(t, err)
		require.NotNil(t, comment)
		assertCommitCommentCount(t, 1, 1)

		createdData, err := comment.GetPushActionContent()
		require.NoError(t, err)
		assert.True(t, createdData.IsForcePush)
		assert.Equal(t, []string{baseCommit.ID.String(), baseCommit.ID.String()}, createdData.CommitIDs)
	})

	t.Run("force-push-ignores-missing-old-commit", func(t *testing.T) {
		require.NoError(t, db.TruncateBeans(t.Context(), &issues_model.Comment{}))
		headCommit, err := gitRepo.GetBranchCommit(pr.HeadBranch)
		require.NoError(t, err)

		commitIDZero := git.Sha1ObjectFormat.EmptyObjectID().String()
		comment, _, err := CreatePushPullComment(t.Context(), pusher, pr, commitIDZero, headCommit.ID.String(), true)
		require.NoError(t, err)
		require.NotNil(t, comment)
		createdData, err := comment.GetPushActionContent()
		require.NoError(t, err)
		assert.True(t, createdData.IsForcePush)
		assert.Equal(t, []string{commitIDZero, headCommit.ID.String()}, createdData.CommitIDs)
		assertCommitCommentCount(t, 2, 1)

		// force push again, the old force push comment should not be deleted, new we have 2 force push comments.
		_, _, err = CreatePushPullComment(t.Context(), pusher, pr, commitIDZero, headCommit.ID.String(), true)
		require.NoError(t, err)
		assertCommitCommentCount(t, 3, 2)
	})

	t.Run("head-vs-base-branch", func(t *testing.T) {
		require.NoError(t, db.TruncateBeans(t.Context(), &issues_model.Comment{}))
		insertCommitComment(t, issues_model.PushActionContent{})
		insertCommitComment(t, issues_model.PushActionContent{})
		insertCommitComment(t, issues_model.PushActionContent{})
		insertCommitComment(t, issues_model.PushActionContent{})
		assertCommitCommentCount(t, 4, 0)

		baseCommit, err := gitRepo.GetBranchCommit(pr.BaseBranch)
		require.NoError(t, err)
		headCommit, err := gitRepo.GetBranchCommit(pr.HeadBranch)
		require.NoError(t, err)

		_, _, err = CreatePushPullComment(t.Context(), pusher, pr, baseCommit.ID.String(), headCommit.ID.String(), true)
		require.NoError(t, err)
		// 2 comments should exist now: one regular push comment and one force-push comment.
		assertCommitCommentCount(t, 2, 1)
	})
}
