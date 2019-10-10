// Copyright 2017 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_newIssueUsers(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	repo := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	newIssue := &Issue{
		RepoID:   repo.ID,
		PosterID: 4,
		Index:    5,
		Title:    "newTestIssueTitle",
		Content:  "newTestIssueContent",
	}

	// artificially insert new issue
	AssertSuccessfulInsert(t, newIssue)

	assert.NoError(t, newIssueUsers(x, repo, newIssue))

	// issue_user table should now have entries for new issue
	AssertExistsAndLoadBean(t, &IssueUser{IssueID: newIssue.ID, UID: newIssue.PosterID})
	AssertExistsAndLoadBean(t, &IssueUser{IssueID: newIssue.ID, UID: repo.OwnerID})
}

func TestUpdateIssueUserByRead(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	issue := AssertExistsAndLoadBean(t, &Issue{ID: 1}).(*Issue)

	assert.NoError(t, UpdateIssueUserByRead(4, issue.ID))
	AssertExistsAndLoadBean(t, &IssueUser{IssueID: issue.ID, UID: 4}, "is_read=1")

	assert.NoError(t, UpdateIssueUserByRead(4, issue.ID))
	AssertExistsAndLoadBean(t, &IssueUser{IssueID: issue.ID, UID: 4}, "is_read=1")

	assert.NoError(t, UpdateIssueUserByRead(NonexistentID, NonexistentID))
}

func TestUpdateIssueUsersByMentions(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	issue := AssertExistsAndLoadBean(t, &Issue{ID: 1}).(*Issue)

	uids := []int64{2, 5}
	assert.NoError(t, UpdateIssueUsersByMentions(DefaultDBContext(), issue.ID, uids))
	for _, uid := range uids {
		AssertExistsAndLoadBean(t, &IssueUser{IssueID: issue.ID, UID: uid}, "is_mentioned=1")
	}
}
