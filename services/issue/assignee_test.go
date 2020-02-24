// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issue

import (
	"testing"

	"code.gitea.io/gitea/models"
	"github.com/stretchr/testify/assert"
)

func TestDeleteNotPassedAssignee(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())

	// Fake issue with assignees
	issue, err := models.GetIssueWithAttrsByID(1)
	assert.NoError(t, err)

	user1, err := models.GetUserByID(1) // This user is already assigned (see the definition in fixtures), so running  UpdateAssignee should unassign him
	assert.NoError(t, err)

	// Check if he got removed
	isAssigned, err := models.IsUserAssignedToIssue(issue, user1)
	assert.NoError(t, err)
	assert.True(t, isAssigned)

	// Clean everyone
	err = DeleteNotPassedAssignee(issue, user1, []*models.User{})
	assert.NoError(t, err)

	// Check they're gone
	assignees, err := models.GetAssigneesByIssue(issue)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(assignees))
}
