// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"testing"

	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowCRUD(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	newWorkflow := func(event WorkflowEvent) *Workflow {
		wf := &Workflow{ProjectID: 1, WorkflowEvent: event, Enabled: true}
		require.NoError(t, CreateWorkflow(ctx, wf))
		return wf
	}
	closed := newWorkflow(WorkflowEventItemClosed)
	opened := newWorkflow(WorkflowEventItemOpened)
	closed2 := newWorkflow(WorkflowEventItemClosed)
	assert.Equal(t, 1, unittest.AssertExistsAndLoadBean(t, &Workflow{ID: closed.ID}).SchemaVersion)

	closed.Enabled = false
	require.NoError(t, UpdateWorkflow(ctx, closed, "enabled"))
	foreign := &Workflow{ID: opened.ID, ProjectID: 2}
	require.NoError(t, UpdateWorkflow(ctx, foreign, "enabled"))
	require.NoError(t, DeleteWorkflow(ctx, 2, opened.ID))
	assert.True(t, unittest.AssertExistsAndLoadBean(t, &Workflow{ID: opened.ID}).Enabled, "other projects cannot modify it")

	workflows, err := FindWorkflowsByProjectID(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, []int64{closed.ID, opened.ID, closed2.ID}, []int64{workflows[0].ID, workflows[1].ID, workflows[2].ID})

	enabled, err := FindEnabledWorkflows(ctx, []int64{1, 2}, WorkflowEventItemClosed)
	require.NoError(t, err)
	assert.Len(t, enabled, 1)
	assert.Equal(t, closed2.ID, enabled[0].ID)

	require.NoError(t, DeleteWorkflow(ctx, 1, closed2.ID))
	unittest.AssertNotExistsBean(t, &Workflow{ID: closed2.ID})

	require.NoError(t, DeleteProjectByID(ctx, 1))
	unittest.AssertNotExistsBean(t, &Workflow{ProjectID: 1})
}
