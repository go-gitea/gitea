// Copyright 2017 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"testing"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_NewIssueUsers(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	newIssue := &issues_model.Issue{
		RepoID:   repo.ID,
		PosterID: 4,
		Index:    6,
		Title:    "newTestIssueTitle",
		Content:  "newTestIssueContent",
	}

	// artificially insert new issue
	require.NoError(t, db.Insert(t.Context(), newIssue))
	require.NoError(t, issues_model.NewIssueUsers(t.Context(), repo, newIssue))

	// issue_user table should now have entries for new issue
	unittest.AssertExistsAndLoadBean(t, &issues_model.IssueUser{IssueID: newIssue.ID, UID: newIssue.PosterID})
	unittest.AssertExistsAndLoadBean(t, &issues_model.IssueUser{IssueID: newIssue.ID, UID: repo.OwnerID})
}

func TestUpdateIssueUserByRead(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})

	assert.NoError(t, issues_model.UpdateIssueUserByRead(t.Context(), 4, issue.ID))
	unittest.AssertExistsAndLoadBean(t, &issues_model.IssueUser{IssueID: issue.ID, UID: 4}, "is_read=1")

	assert.NoError(t, issues_model.UpdateIssueUserByRead(t.Context(), 4, issue.ID))
	unittest.AssertExistsAndLoadBean(t, &issues_model.IssueUser{IssueID: issue.ID, UID: 4}, "is_read=1")

	assert.NoError(t, issues_model.UpdateIssueUserByRead(t.Context(), unittest.NonexistentID, unittest.NonexistentID))
}

func TestUpdateIssueUsersByMentions(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})

	uids := []int64{2, 5}
	assert.NoError(t, issues_model.UpdateIssueUsersByMentions(t.Context(), issue.ID, uids))
	for _, uid := range uids {
		unittest.AssertExistsAndLoadBean(t, &issues_model.IssueUser{IssueID: issue.ID, UID: uid}, "is_mentioned=1")
	}
}
