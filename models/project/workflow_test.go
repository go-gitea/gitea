// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"testing"

	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestSaveWorkflow(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	assert.NoError(t, SaveWorkflow(t.Context(), 1, WorkflowEventItemClosed, 3, true))
	w, err := GetEnabledWorkflow(t.Context(), 1, WorkflowEventItemClosed)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), w.ColumnID)

	// saving again updates the existing row
	assert.NoError(t, SaveWorkflow(t.Context(), 1, WorkflowEventItemClosed, 2, false))
	workflows, err := GetWorkflowsByProjectID(t.Context(), 1)
	assert.NoError(t, err)
	assert.Len(t, workflows, 1)
	assert.Equal(t, int64(2), workflows[WorkflowEventItemClosed].ColumnID)
	w, err = GetEnabledWorkflow(t.Context(), 1, WorkflowEventItemClosed)
	assert.NoError(t, err)
	assert.Nil(t, w)

	assert.Error(t, SaveWorkflow(t.Context(), 1, "unknown", 2, true))
	assert.Error(t, SaveWorkflow(t.Context(), 1, WorkflowEventItemClosed, 4, true), "column of another project")
}

func TestGetColumnForNewItem(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	project, err := GetProjectByID(t.Context(), 1)
	assert.NoError(t, err)

	column, err := project.GetColumnForNewItem(t.Context())
	assert.NoError(t, err)
	assert.Equal(t, int64(1), column.ID, "default column")

	assert.NoError(t, SaveWorkflow(t.Context(), 1, WorkflowEventItemAdded, 2, true))
	column, err = project.GetColumnForNewItem(t.Context())
	assert.NoError(t, err)
	assert.Equal(t, int64(2), column.ID)
}

func TestWorkflowCleanup(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	assert.NoError(t, SaveWorkflow(t.Context(), 1, WorkflowEventItemClosed, 2, true))
	assert.NoError(t, SaveWorkflow(t.Context(), 1, WorkflowEventItemReopened, 3, true))

	assert.NoError(t, DeleteColumnByID(t.Context(), 2))
	workflows, err := GetWorkflowsByProjectID(t.Context(), 1)
	assert.NoError(t, err)
	assert.Len(t, workflows, 1)
	assert.Contains(t, workflows, WorkflowEventItemReopened)

	assert.NoError(t, DeleteProjectByID(t.Context(), 1))
	workflows, err = GetWorkflowsByProjectID(t.Context(), 1)
	assert.NoError(t, err)
	assert.Empty(t, workflows)
}
