// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issue

import (
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestDeleteNotPassedAssignee(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Fake issue with assignees
	issue, err := models.GetIssueWithAttrsByID(1)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, len(issue.Assignees))

	user1, err := user_model.GetUserByID(1) // This user is already assigned (see the definition in fixtures), so running  UpdateAssignee should unassign him
	assert.NoError(t, err)

	// Check if he got removed
	isAssigned, err := models.IsUserAssignedToIssue(db.DefaultContext, issue, user1)
	assert.NoError(t, err)
	assert.True(t, isAssigned)

	// Clean everyone
	err = DeleteNotPassedAssignee(issue, user1, []*user_model.User{})
	assert.NoError(t, err)
	assert.EqualValues(t, 0, len(issue.Assignees))

	// Check they're gone
	assert.NoError(t, issue.LoadAssignees(db.DefaultContext))
	assert.EqualValues(t, 0, len(issue.Assignees))
	assert.Empty(t, issue.Assignee)
}
