// Copyright 2017 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
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
	unittest.AssertSuccessfulInsert(t, newIssue)

	assert.NoError(t, issues_model.NewIssueUsers(db.DefaultContext, repo, newIssue))

	// issue_user table should now have entries for new issue
	unittest.AssertExistsAndLoadBean(t, &issues_model.IssueUser{IssueID: newIssue.ID, UID: newIssue.PosterID})
	unittest.AssertExistsAndLoadBean(t, &issues_model.IssueUser{IssueID: newIssue.ID, UID: repo.OwnerID})
}

func TestUpdateIssueUserByRead(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})

	assert.NoError(t, issues_model.UpdateIssueUserByRead(db.DefaultContext, 4, issue.ID))
	unittest.AssertExistsAndLoadBean(t, &issues_model.IssueUser{IssueID: issue.ID, UID: 4}, "is_read=1")

	assert.NoError(t, issues_model.UpdateIssueUserByRead(db.DefaultContext, 4, issue.ID))
	unittest.AssertExistsAndLoadBean(t, &issues_model.IssueUser{IssueID: issue.ID, UID: 4}, "is_read=1")

	assert.NoError(t, issues_model.UpdateIssueUserByRead(db.DefaultContext, unittest.NonexistentID, unittest.NonexistentID))
}

func TestUpdateIssueUsersByMentions(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})

	uids := []int64{2, 5}
	assert.NoError(t, issues_model.UpdateIssueUsersByMentions(db.DefaultContext, issue.ID, uids))
	for _, uid := range uids {
		unittest.AssertExistsAndLoadBean(t, &issues_model.IssueUser{IssueID: issue.ID, UID: uid}, "is_mentioned=1")
	}
}
