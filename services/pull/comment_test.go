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

	commits, err := gitRepo.CommitsBetweenNotBase(headCommit, oldCommit, pr.BaseBranch)
	assert.NoError(t, err)
	expectedCount := 1
	if len(commits) > 0 {
		expectedCount = 2
	}

	comments, err = issues_model.FindComments(t.Context(), &issues_model.FindCommentsOptions{
		IssueID: pr.IssueID,
		Type:    issues_model.CommentTypePullRequestPush,
	})
	assert.NoError(t, err)
	assert.Len(t, comments, expectedCount)

	forcePushCount := 0
	for _, comment := range comments {
		var pushData issues_model.PushActionContent
		assert.NoError(t, json.Unmarshal([]byte(comment.Content), &pushData))
		if pushData.IsForcePush {
			forcePushCount++
		}
	}
	assert.Equal(t, 1, forcePushCount)
}
