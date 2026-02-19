// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"testing"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/json"

	"github.com/stretchr/testify/assert"
)

func TestCreatePushPullCommentForcePushDeletesOldComments(t *testing.T) {
	t.Run("base-branch-only", func(t *testing.T) {
		assert.NoError(t, unittest.PrepareTestDatabase())

		pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
		assert.NoError(t, pr.LoadIssue(t.Context()))
		assert.NoError(t, pr.LoadBaseRepo(t.Context()))

		pusher := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

		_, err := issues_model.CreateComment(t.Context(), &issues_model.CreateCommentOptions{
			Type:    issues_model.CommentTypePullRequestPush,
			Doer:    pusher,
			Repo:    pr.BaseRepo,
			Issue:   pr.Issue,
			Content: "{}",
		})
		assert.NoError(t, err)
		_, err = issues_model.CreateComment(t.Context(), &issues_model.CreateCommentOptions{
			Type:    issues_model.CommentTypePullRequestPush,
			Doer:    pusher,
			Repo:    pr.BaseRepo,
			Issue:   pr.Issue,
			Content: "{}",
		})
		assert.NoError(t, err)

		comments, err := issues_model.FindComments(t.Context(), &issues_model.FindCommentsOptions{
			IssueID: pr.IssueID,
			Type:    issues_model.CommentTypePullRequestPush,
		})
		assert.NoError(t, err)
		assert.Len(t, comments, 2)

		gitRepo, err := gitrepo.OpenRepository(t.Context(), pr.BaseRepo)
		assert.NoError(t, err)
		defer gitRepo.Close()

		headCommit, err := gitRepo.GetBranchCommit(pr.BaseBranch)
		assert.NoError(t, err)
		oldCommit := headCommit
		if headCommit.ParentCount() > 0 {
			parentCommit, err := headCommit.Parent(0)
			assert.NoError(t, err)
			oldCommit = parentCommit
		}

		comment, err := CreatePushPullComment(t.Context(), pusher, pr, oldCommit.ID.String(), headCommit.ID.String(), true)
		assert.NoError(t, err)
		assert.NotNil(t, comment)
		var createdData issues_model.PushActionContent
		assert.NoError(t, json.Unmarshal([]byte(comment.Content), &createdData))
		assert.True(t, createdData.IsForcePush)

		// When both commits are on the base branch, CommitsBetweenNotBase should
		// typically return no commits, so only the force-push comment is expected.
		commits, err := gitRepo.CommitsBetweenNotBase(headCommit, oldCommit, pr.BaseBranch)
		assert.NoError(t, err)
		assert.Empty(t, commits)

		comments, err = issues_model.FindComments(t.Context(), &issues_model.FindCommentsOptions{
			IssueID: pr.IssueID,
			Type:    issues_model.CommentTypePullRequestPush,
		})
		assert.NoError(t, err)
		assert.Len(t, comments, 1)

		forcePushCount := 0
		for _, comment := range comments {
			var pushData issues_model.PushActionContent
			assert.NoError(t, json.Unmarshal([]byte(comment.Content), &pushData))
			if pushData.IsForcePush {
				forcePushCount++
			}
		}
		assert.Equal(t, 1, forcePushCount)
	})

	t.Run("head-vs-base-branch", func(t *testing.T) {
		assert.NoError(t, unittest.PrepareTestDatabase())

		pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
		assert.NoError(t, pr.LoadIssue(t.Context()))
		assert.NoError(t, pr.LoadBaseRepo(t.Context()))

		pusher := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

		_, err := issues_model.CreateComment(t.Context(), &issues_model.CreateCommentOptions{
			Type:    issues_model.CommentTypePullRequestPush,
			Doer:    pusher,
			Repo:    pr.BaseRepo,
			Issue:   pr.Issue,
			Content: "{}",
		})
		assert.NoError(t, err)
		_, err = issues_model.CreateComment(t.Context(), &issues_model.CreateCommentOptions{
			Type:    issues_model.CommentTypePullRequestPush,
			Doer:    pusher,
			Repo:    pr.BaseRepo,
			Issue:   pr.Issue,
			Content: "{}",
		})
		assert.NoError(t, err)

		comments, err := issues_model.FindComments(t.Context(), &issues_model.FindCommentsOptions{
			IssueID: pr.IssueID,
			Type:    issues_model.CommentTypePullRequestPush,
		})
		assert.NoError(t, err)
		assert.Len(t, comments, 2)

		gitRepo, err := gitrepo.OpenRepository(t.Context(), pr.BaseRepo)
		assert.NoError(t, err)
		defer gitRepo.Close()

		// In this subtest, use the head branch for the new commit and the base branch
		// for the old commit so that CommitsBetweenNotBase returns non-empty results.
		headCommit, err := gitRepo.GetBranchCommit(pr.HeadBranch)
		assert.NoError(t, err)

		baseCommit, err := gitRepo.GetBranchCommit(pr.BaseBranch)
		assert.NoError(t, err)
		oldCommit := baseCommit

		comment, err := CreatePushPullComment(t.Context(), pusher, pr, oldCommit.ID.String(), headCommit.ID.String(), true)
		assert.NoError(t, err)
		assert.NotNil(t, comment)
		var createdData issues_model.PushActionContent
		assert.NoError(t, json.Unmarshal([]byte(comment.Content), &createdData))
		assert.True(t, createdData.IsForcePush)

		commits, err := gitRepo.CommitsBetweenNotBase(headCommit, oldCommit, pr.BaseBranch)
		assert.NoError(t, err)
		// For this scenario we expect at least one commit between head and base
		// that is not on the base branch, so data.CommitIDs should be non-empty.
		assert.NotEmpty(t, commits)

		comments, err = issues_model.FindComments(t.Context(), &issues_model.FindCommentsOptions{
			IssueID: pr.IssueID,
			Type:    issues_model.CommentTypePullRequestPush,
		})
		assert.NoError(t, err)
		// Two comments should exist now: one regular push comment and one force-push comment.
		assert.Len(t, comments, 2)

		forcePushCount := 0
		for _, comment := range comments {
			var pushData issues_model.PushActionContent
			assert.NoError(t, json.Unmarshal([]byte(comment.Content), &pushData))
			if pushData.IsForcePush {
				forcePushCount++
			}
		}
		assert.Equal(t, 1, forcePushCount)
	})
}
