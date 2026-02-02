// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"testing"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestChangeTitleWIPPrefix(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Load a pull request
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 1})
	assert.NoError(t, pr.LoadIssue(t.Context()))
	issue := pr.Issue

	// Load repo and doer
	assert.NoError(t, issue.LoadRepo(t.Context()))
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: issue.Repo.OwnerID})

	// Get the WIP prefix from settings
	wipPrefix := setting.Repository.PullRequest.WorkInProgressPrefixes[0]

	// Store original title
	originalTitle := issue.Title
	wipTitle := wipPrefix + " " + originalTitle

	// Test 1: Add WIP prefix
	err := ChangeTitle(t.Context(), issue, doer, wipTitle)
	assert.NoError(t, err)

	// Check that a comment was created with the correct type
	comments, err := issues_model.FindComments(t.Context(), &issues_model.FindCommentsOptions{
		IssueID: issue.ID,
		Type:    issues_model.CommentTypeMarkedAsWorkInProgress,
	})
	assert.NoError(t, err)
	assert.Len(t, comments, 1, "Should have created a CommentTypeMarkedAsWorkInProgress comment")

	// Test 2: Remove WIP prefix
	err = ChangeTitle(t.Context(), issue, doer, originalTitle)
	assert.NoError(t, err)

	// Check that a comment was created with the correct type
	comments, err = issues_model.FindComments(t.Context(), &issues_model.FindCommentsOptions{
		IssueID: issue.ID,
		Type:    issues_model.CommentTypeMarkedAsReadyForReview,
	})
	assert.NoError(t, err)
	assert.Len(t, comments, 1, "Should have created a CommentTypeMarkedAsReadyForReview comment")
}

func TestChangeTitleNormalChange(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Load a pull request
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 1})
	assert.NoError(t, pr.LoadIssue(t.Context()))
	issue := pr.Issue

	// Load repo and doer
	assert.NoError(t, issue.LoadRepo(t.Context()))
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: issue.Repo.OwnerID})

	// Store original title
	originalTitle := issue.Title
	newTitle := "New title without WIP"

	// Ensure neither title has WIP prefix
	assert.False(t, issues_model.HasWorkInProgressPrefix(originalTitle))
	assert.False(t, issues_model.HasWorkInProgressPrefix(newTitle))

	// Change title
	err := ChangeTitle(t.Context(), issue, doer, newTitle)
	assert.NoError(t, err)

	// Check that a normal change title comment was created
	comments, err := issues_model.FindComments(t.Context(), &issues_model.FindCommentsOptions{
		IssueID: issue.ID,
		Type:    issues_model.CommentTypeChangeTitle,
	})
	assert.NoError(t, err)
	assert.Greater(t, len(comments), 0, "Should have created a CommentTypeChangeTitle comment")

	// Verify the last comment has the correct old and new titles
	lastComment := comments[len(comments)-1]
	assert.Equal(t, originalTitle, lastComment.OldTitle)
	assert.Equal(t, newTitle, lastComment.NewTitle)
}
