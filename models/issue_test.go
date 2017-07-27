// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIssue_ReplaceLabels(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	testSuccess := func(issueID int64, labelIDs []int64) {
		issue := AssertExistsAndLoadBean(t, &Issue{ID: issueID}).(*Issue)
		repo := AssertExistsAndLoadBean(t, &Repository{ID: issue.RepoID}).(*Repository)
		doer := AssertExistsAndLoadBean(t, &User{ID: repo.OwnerID}).(*User)

		labels := make([]*Label, len(labelIDs))
		for i, labelID := range labelIDs {
			labels[i] = AssertExistsAndLoadBean(t, &Label{ID: labelID, RepoID: repo.ID}).(*Label)
		}
		assert.NoError(t, issue.ReplaceLabels(labels, doer))
		AssertCount(t, &IssueLabel{IssueID: issueID}, len(labelIDs))
		for _, labelID := range labelIDs {
			AssertExistsAndLoadBean(t, &IssueLabel{IssueID: issueID, LabelID: labelID})
		}
	}

	testSuccess(1, []int64{2})
	testSuccess(1, []int64{1, 2})
	testSuccess(1, []int64{})
}

func TestIssueAPIURL(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	issue := AssertExistsAndLoadBean(t, &Issue{ID: 1}).(*Issue)
	err := issue.LoadAttributes()

	assert.NoError(t, err)
	assert.Equal(t, "https://try.gitea.io/api/v1/repos/user2/repo1/issues/1", issue.APIURL())
}

func TestGetIssuesByIDs(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	testSuccess := func(expectedIssueIDs []int64, nonExistentIssueIDs []int64) {
		issues, err := GetIssuesByIDs(append(expectedIssueIDs, nonExistentIssueIDs...))
		assert.NoError(t, err)
		actualIssueIDs := make([]int64, len(issues))
		for i, issue := range issues {
			actualIssueIDs[i] = issue.ID
		}
		assert.Equal(t, expectedIssueIDs, actualIssueIDs)

	}
	testSuccess([]int64{1, 2, 3}, []int64{})
	testSuccess([]int64{1, 2, 3}, []int64{NonexistentID})
}

func TestGetParticipantsByIssueID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	checkParticipants := func(issueID int64, userIDs []int) {
		participants, err := GetParticipantsByIssueID(issueID)
		if assert.NoError(t, err) {
			participantsIDs := make([]int, len(participants))
			for i, u := range participants {
				participantsIDs[i] = int(u.ID)
			}
			sort.Ints(participantsIDs)
			sort.Ints(userIDs)
			assert.Equal(t, userIDs, participantsIDs)
		}
	}

	// User 1 is issue1 poster (see fixtures/issue.yml)
	// User 2 only labeled issue1 (see fixtures/comment.yml)
	// Users 3 and 5 made actual comments (see fixtures/comment.yml)
	checkParticipants(1, []int{3, 5})
}

func TestIssue_AddLabel(t *testing.T) {
	var tests = []struct {
		issueID int64
		labelID int64
		doerID  int64
	}{
		{1, 2, 2}, // non-pull-request, not-already-added label
		{1, 1, 2}, // non-pull-request, already-added label
		{2, 2, 2}, // pull-request, not-already-added label
		{2, 1, 2}, // pull-request, already-added label
	}
	for _, test := range tests {
		assert.NoError(t, PrepareTestDatabase())
		issue := AssertExistsAndLoadBean(t, &Issue{ID: test.issueID}).(*Issue)
		label := AssertExistsAndLoadBean(t, &Label{ID: test.labelID}).(*Label)
		doer := AssertExistsAndLoadBean(t, &User{ID: test.doerID}).(*User)
		assert.NoError(t, issue.AddLabel(doer, label))
		AssertExistsAndLoadBean(t, &IssueLabel{IssueID: test.issueID, LabelID: test.labelID})
	}
}

func TestIssue_AddLabels(t *testing.T) {
	var tests = []struct {
		issueID  int64
		labelIDs []int64
		doerID   int64
	}{
		{1, []int64{1, 2}, 2}, // non-pull-request
		{1, []int64{}, 2},     // non-pull-request, empty
		{2, []int64{1, 2}, 2}, // pull-request
		{2, []int64{}, 1},     // pull-request, empty
	}
	for _, test := range tests {
		assert.NoError(t, PrepareTestDatabase())
		issue := AssertExistsAndLoadBean(t, &Issue{ID: test.issueID}).(*Issue)
		labels := make([]*Label, len(test.labelIDs))
		for i, labelID := range test.labelIDs {
			labels[i] = AssertExistsAndLoadBean(t, &Label{ID: labelID}).(*Label)
		}
		doer := AssertExistsAndLoadBean(t, &User{ID: test.doerID}).(*User)
		assert.NoError(t, issue.AddLabels(doer, labels))
		for _, labelID := range test.labelIDs {
			AssertExistsAndLoadBean(t, &IssueLabel{IssueID: test.issueID, LabelID: labelID})
		}
	}
}

func TestIssue_ClearLabels(t *testing.T) {
	var tests = []struct {
		issueID int64
		doerID  int64
	}{
		{1, 2}, // non-pull-request, has labels
		{2, 2}, // pull-request, has labels
		{3, 2}, // pull-request, has no labels
	}
	for _, test := range tests {
		assert.NoError(t, PrepareTestDatabase())
		issue := AssertExistsAndLoadBean(t, &Issue{ID: test.issueID}).(*Issue)
		doer := AssertExistsAndLoadBean(t, &User{ID: test.doerID}).(*User)
		assert.NoError(t, issue.ClearLabels(doer))
		AssertNotExistsBean(t, &IssueLabel{IssueID: test.issueID})
	}
}

func TestUpdateIssueCols(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	issue := AssertExistsAndLoadBean(t, &Issue{}).(*Issue)

	const newTitle = "New Title for unit test"
	issue.Title = newTitle

	prevContent := issue.Content
	issue.Content = "This should have no effect"

	now := time.Now().Unix()
	assert.NoError(t, UpdateIssueCols(issue, "name"))
	then := time.Now().Unix()

	updatedIssue := AssertExistsAndLoadBean(t, &Issue{ID: issue.ID}).(*Issue)
	assert.EqualValues(t, newTitle, updatedIssue.Title)
	assert.EqualValues(t, prevContent, updatedIssue.Content)
	AssertInt64InRange(t, now, then, updatedIssue.UpdatedUnix)
}
