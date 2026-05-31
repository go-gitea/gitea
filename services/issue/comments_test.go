// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"testing"

	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateIssueCommentSkipsConsecutiveDuplicate(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issue.RepoID})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	issue.Repo = repo

	before, err := issues_model.CountComments(t.Context(), &issues_model.FindCommentsOptions{
		IssueID: issue.ID,
		Type:    issues_model.CommentTypeComment,
	})
	require.NoError(t, err)

	first, err := CreateIssueComment(t.Context(), doer, repo, issue, "duplicate comment", nil)
	require.NoError(t, err)
	second, err := CreateIssueComment(t.Context(), doer, repo, issue, "duplicate comment", nil)
	require.NoError(t, err)

	after, err := issues_model.CountComments(t.Context(), &issues_model.FindCommentsOptions{
		IssueID: issue.ID,
		Type:    issues_model.CommentTypeComment,
	})
	require.NoError(t, err)

	assert.Equal(t, first.ID, second.ID)
	assert.Equal(t, before+1, after)
}
