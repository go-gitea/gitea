// Copyright 2017 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func Test_newIssueUsers(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}).(*repo_model.Repository)
	newIssue := &Issue{
		RepoID:   repo.ID,
		PosterID: 4,
		Index:    6,
		Title:    "newTestIssueTitle",
		Content:  "newTestIssueContent",
	}

	// artificially insert new issue
	unittest.AssertSuccessfulInsert(t, newIssue)

	assert.NoError(t, newIssueUsers(db.DefaultContext, repo, newIssue))

	// issue_user table should now have entries for new issue
	unittest.AssertExistsAndLoadBean(t, &IssueUser{IssueID: newIssue.ID, UID: newIssue.PosterID})
	unittest.AssertExistsAndLoadBean(t, &IssueUser{IssueID: newIssue.ID, UID: repo.OwnerID})
}

func TestUpdateIssueUserByRead(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue := unittest.AssertExistsAndLoadBean(t, &Issue{ID: 1}).(*Issue)

	assert.NoError(t, UpdateIssueUserByRead(4, issue.ID))
	unittest.AssertExistsAndLoadBean(t, &IssueUser{IssueID: issue.ID, UID: 4}, "is_read=1")

	assert.NoError(t, UpdateIssueUserByRead(4, issue.ID))
	unittest.AssertExistsAndLoadBean(t, &IssueUser{IssueID: issue.ID, UID: 4}, "is_read=1")

	assert.NoError(t, UpdateIssueUserByRead(unittest.NonexistentID, unittest.NonexistentID))
}

func TestUpdateIssueUsersByMentions(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue := unittest.AssertExistsAndLoadBean(t, &Issue{ID: 1}).(*Issue)

	uids := []int64{2, 5}
	assert.NoError(t, UpdateIssueUsersByMentions(db.DefaultContext, issue.ID, uids))
	for _, uid := range uids {
		unittest.AssertExistsAndLoadBean(t, &IssueUser{IssueID: issue.ID, UID: uid}, "is_mentioned=1")
	}
}
