// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestUpdateAssignee(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Fake issue with assignees
	issue, err := GetIssueWithAttrsByID(1)
	assert.NoError(t, err)

	// Assign multiple users
	user2, err := user_model.GetUserByID(2)
	assert.NoError(t, err)
	_, _, err = ToggleIssueAssignee(issue, &user_model.User{ID: 1}, user2.ID)
	assert.NoError(t, err)

	user3, err := user_model.GetUserByID(3)
	assert.NoError(t, err)
	_, _, err = ToggleIssueAssignee(issue, &user_model.User{ID: 1}, user3.ID)
	assert.NoError(t, err)

	user1, err := user_model.GetUserByID(1) // This user is already assigned (see the definition in fixtures), so running  UpdateAssignee should unassign him
	assert.NoError(t, err)
	_, _, err = ToggleIssueAssignee(issue, &user_model.User{ID: 1}, user1.ID)
	assert.NoError(t, err)

	// Check if he got removed
	isAssigned, err := IsUserAssignedToIssue(issue, user1)
	assert.NoError(t, err)
	assert.False(t, isAssigned)

	// Check if they're all there
	assignees, err := GetAssigneesByIssue(issue)
	assert.NoError(t, err)

	var expectedAssignees []*user_model.User
	expectedAssignees = append(expectedAssignees, user2, user3)

	for in, assignee := range assignees {
		assert.Equal(t, assignee.ID, expectedAssignees[in].ID)
	}

	// Check if the user is assigned
	isAssigned, err = IsUserAssignedToIssue(issue, user2)
	assert.NoError(t, err)
	assert.True(t, isAssigned)

	// This user should not be assigned
	isAssigned, err = IsUserAssignedToIssue(issue, &user_model.User{ID: 4})
	assert.NoError(t, err)
	assert.False(t, isAssigned)
}

func TestMakeIDsFromAPIAssigneesToAdd(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	_ = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}).(*user_model.User)
	_ = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)

	IDs, err := MakeIDsFromAPIAssigneesToAdd("", []string{""})
	assert.NoError(t, err)
	assert.Equal(t, []int64{}, IDs)

	_, err = MakeIDsFromAPIAssigneesToAdd("", []string{"none_existing_user"})
	assert.Error(t, err)

	IDs, err = MakeIDsFromAPIAssigneesToAdd("user1", []string{"user1"})
	assert.NoError(t, err)
	assert.Equal(t, []int64{1}, IDs)

	IDs, err = MakeIDsFromAPIAssigneesToAdd("user2", []string{""})
	assert.NoError(t, err)
	assert.Equal(t, []int64{2}, IDs)

	IDs, err = MakeIDsFromAPIAssigneesToAdd("", []string{"user1", "user2"})
	assert.NoError(t, err)
	assert.Equal(t, []int64{1, 2}, IDs)
}
