// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"testing"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
)

func TestDeleteNotPassedAssignee(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Fake issue with assignees
	issue, err := issues_model.GetIssueByID(t.Context(), 1)
	assert.NoError(t, err)

	err = issue.LoadAttributes(t.Context())
	assert.NoError(t, err)

	assert.Len(t, issue.Assignees, 1)

	user1, err := user_model.GetUserByID(t.Context(), 1) // This user is already assigned (see the definition in fixtures), so running  UpdateAssignee should unassign him
	assert.NoError(t, err)

	// Check if he got removed
	isAssigned, err := issues_model.IsUserAssignedToIssue(t.Context(), issue, user1.ID)
	assert.NoError(t, err)
	assert.True(t, isAssigned)

	// Clean everyone
	err = DeleteNotPassedAssignee(t.Context(), issue, user1, []*user_model.User{})
	assert.NoError(t, err)
	assert.Empty(t, issue.Assignees)

	// Reload to check they're gone
	issue.ResetAttributesLoaded()
	assert.NoError(t, issue.LoadAssignees(t.Context()))
	assert.Empty(t, issue.Assignees)
	assert.Empty(t, issue.Assignee)
}

func TestAddAssigneeIfNotAssignedBlocked(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	issue, err := issues_model.GetIssueByID(t.Context(), 1)
	assert.NoError(t, err)
	assert.NoError(t, issue.LoadRepo(t.Context()))

	doer, err := user_model.GetUserByID(t.Context(), 4)
	assert.NoError(t, err)

	assignee, err := user_model.GetUserByID(t.Context(), 2)
	assert.NoError(t, err)

	assert.NoError(t, db.Insert(t.Context(), &user_model.Blocking{
		BlockerID: assignee.ID,
		BlockeeID: doer.ID,
	}))

	_, err = AddAssigneeIfNotAssigned(t.Context(), issue, doer, assignee)
	assert.ErrorIs(t, err, user_model.ErrBlockedUser)

	isAssigned, err := issues_model.IsUserAssignedToIssue(t.Context(), issue, assignee.ID)
	assert.NoError(t, err)
	assert.False(t, isAssigned)
}

func TestAddAssigneesBlockedIsAtomic(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	issue, err := issues_model.GetIssueByID(t.Context(), 1)
	assert.NoError(t, err)
	assert.NoError(t, issue.LoadAttributes(t.Context()))

	doer, err := user_model.GetUserByID(t.Context(), 2)
	assert.NoError(t, err)

	blockedAssignee, err := user_model.GetUserByID(t.Context(), 40)
	assert.NoError(t, err)

	assert.NoError(t, db.Insert(t.Context(), &user_model.Blocking{
		BlockerID: blockedAssignee.ID,
		BlockeeID: doer.ID,
	}))

	err = AddAssignees(t.Context(), issue, doer, []int64{doer.ID, blockedAssignee.ID})
	assert.ErrorIs(t, err, user_model.ErrBlockedUser)

	assigneeIDs, err := issues_model.GetAssigneeIDsByIssue(t.Context(), issue.ID)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []int64{1}, assigneeIDs)
}
