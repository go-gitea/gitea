// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"testing"

	issues_model "gitea.dev/models/issues"
	project_model "gitea.dev/models/project"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
)

func TestMoveIssuesClosesIssue(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	assert.False(t, issue.IsClosed)
	column2 := unittest.AssertExistsAndLoadBean(t, &project_model.Column{ID: 2})
	column3 := unittest.AssertExistsAndLoadBean(t, &project_model.Column{ID: 3})

	assert.NoError(t, project_model.SaveWorkflow(t.Context(), 1, project_model.WorkflowEventCardMoved, 3, true))

	assert.NoError(t, MoveIssuesOnProjectColumn(t.Context(), doer, column2, map[int64]int64{0: issue.ID}))
	issue = unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	assert.False(t, issue.IsClosed, "a column without the workflow keeps the issue open")

	assert.NoError(t, MoveIssuesOnProjectColumn(t.Context(), doer, column3, map[int64]int64{0: issue.ID}))
	issue = unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	assert.True(t, issue.IsClosed)
}
