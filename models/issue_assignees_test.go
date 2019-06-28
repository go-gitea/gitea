// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpdateAssignee(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	// Fake issue with assignees
	issue, err := GetIssueWithAttrsByID(1)
	assert.NoError(t, err)

	// Assign multiple users
	user2, err := GetUserByID(2)
	assert.NoError(t, err)
	err = UpdateAssignee(issue, &User{ID: 1}, user2.ID)
	assert.NoError(t, err)

	user3, err := GetUserByID(3)
	assert.NoError(t, err)
	err = UpdateAssignee(issue, &User{ID: 1}, user3.ID)
	assert.NoError(t, err)

	user1, err := GetUserByID(1) // This user is already assigned (see the definition in fixtures), so running  UpdateAssignee should unassign him
	assert.NoError(t, err)
	err = UpdateAssignee(issue, &User{ID: 1}, user1.ID)
	assert.NoError(t, err)

	// Check if he got removed
	isAssigned, err := IsUserAssignedToIssue(issue, user1)
	assert.NoError(t, err)
	assert.False(t, isAssigned)

	// Check if they're all there
	assignees, err := GetAssigneesByIssue(issue)
	assert.NoError(t, err)

	var expectedAssignees []*User
	expectedAssignees = append(expectedAssignees, user2, user3)

	for in, assignee := range assignees {
		assert.Equal(t, assignee.ID, expectedAssignees[in].ID)
	}

	// Check if the user is assigned
	isAssigned, err = IsUserAssignedToIssue(issue, user2)
	assert.NoError(t, err)
	assert.True(t, isAssigned)

	// This user should not be assigned
	isAssigned, err = IsUserAssignedToIssue(issue, &User{ID: 4})
	assert.NoError(t, err)
	assert.False(t, isAssigned)

	// Clean everyone
	err = DeleteNotPassedAssignee(issue, user1, []*User{})
	assert.NoError(t, err)

	// Check they're gone
	assignees, err = GetAssigneesByIssue(issue)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(assignees))
}
