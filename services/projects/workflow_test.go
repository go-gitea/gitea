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

func TestApplyWorkflows(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1}) // in column 1 of project 1

	columnOf := func() int64 {
		columnMap, err := issue.ProjectColumnMap(t.Context())
		assert.NoError(t, err)
		return columnMap[1]
	}

	// no workflow: the card stays
	ApplyWorkflows(t.Context(), doer, issue, project_model.WorkflowEventItemClosed)
	assert.Equal(t, int64(1), columnOf())

	assert.NoError(t, project_model.SaveWorkflow(t.Context(), 1, project_model.WorkflowEventItemClosed, 3, true))
	assert.NoError(t, project_model.SaveWorkflow(t.Context(), 1, project_model.WorkflowEventItemReopened, 2, false))

	ApplyWorkflows(t.Context(), doer, issue, project_model.WorkflowEventItemReopened)
	assert.Equal(t, int64(1), columnOf(), "disabled workflow")

	ApplyWorkflows(t.Context(), doer, issue, project_model.WorkflowEventItemClosed)
	assert.Equal(t, int64(3), columnOf())
}
