// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestUpdateAssignee(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Fake issue with assignees
	issue, err := issues_model.GetIssueWithAttrsByID(1)
	assert.NoError(t, err)

	// Assign multiple users
	user2, err := user_model.GetUserByID(db.DefaultContext, 2)
	assert.NoError(t, err)
	_, _, err = issues_model.ToggleIssueAssignee(db.DefaultContext, issue, &user_model.User{ID: 1}, user2.ID)
	assert.NoError(t, err)

	user3, err := user_model.GetUserByID(db.DefaultContext, 3)
	assert.NoError(t, err)
	_, _, err = issues_model.ToggleIssueAssignee(db.DefaultContext, issue, &user_model.User{ID: 1}, user3.ID)
	assert.NoError(t, err)

	user1, err := user_model.GetUserByID(db.DefaultContext, 1) // This user is already assigned (see the definition in fixtures), so running  UpdateAssignee should unassign him
	assert.NoError(t, err)
	_, _, err = issues_model.ToggleIssueAssignee(db.DefaultContext, issue, &user_model.User{ID: 1}, user1.ID)
	assert.NoError(t, err)

	// Check if he got removed
	isAssigned, err := issues_model.IsUserAssignedToIssue(db.DefaultContext, issue, user1)
	assert.NoError(t, err)
	assert.False(t, isAssigned)

	// Check if they're all there
	err = issue.LoadAssignees(db.DefaultContext)
	assert.NoError(t, err)

	var expectedAssignees []*user_model.User
	expectedAssignees = append(expectedAssignees, user2, user3)

	for in, assignee := range issue.Assignees {
		assert.Equal(t, assignee.ID, expectedAssignees[in].ID)
	}

	// Check if the user is assigned
	isAssigned, err = issues_model.IsUserAssignedToIssue(db.DefaultContext, issue, user2)
	assert.NoError(t, err)
	assert.True(t, isAssigned)

	// This user should not be assigned
	isAssigned, err = issues_model.IsUserAssignedToIssue(db.DefaultContext, issue, &user_model.User{ID: 4})
	assert.NoError(t, err)
	assert.False(t, isAssigned)
}

func TestMakeIDsFromAPIAssigneesToAdd(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	_ = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	_ = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	IDs, err := issues_model.MakeIDsFromAPIAssigneesToAdd(db.DefaultContext, "", []string{""})
	assert.NoError(t, err)
	assert.Equal(t, []int64{}, IDs)

	_, err = issues_model.MakeIDsFromAPIAssigneesToAdd(db.DefaultContext, "", []string{"none_existing_user"})
	assert.Error(t, err)

	IDs, err = issues_model.MakeIDsFromAPIAssigneesToAdd(db.DefaultContext, "user1", []string{"user1"})
	assert.NoError(t, err)
	assert.Equal(t, []int64{1}, IDs)

	IDs, err = issues_model.MakeIDsFromAPIAssigneesToAdd(db.DefaultContext, "user2", []string{""})
	assert.NoError(t, err)
	assert.Equal(t, []int64{2}, IDs)

	IDs, err = issues_model.MakeIDsFromAPIAssigneesToAdd(db.DefaultContext, "", []string{"user1", "user2"})
	assert.NoError(t, err)
	assert.Equal(t, []int64{1, 2}, IDs)
}
