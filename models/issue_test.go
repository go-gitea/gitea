// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"sort"
	"testing"

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
