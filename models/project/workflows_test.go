// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"testing"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
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
	added := newWorkflow(WorkflowEventItemAddedToProject)
	closed2 := newWorkflow(WorkflowEventItemClosed)

	closed.Enabled = false
	require.NoError(t, UpdateWorkflow(ctx, closed, "enabled"))
	require.NoError(t, UpdateWorkflow(ctx, &Workflow{ID: added.ID, ProjectID: 2}, "enabled"))
	require.NoError(t, DeleteWorkflow(ctx, 2, added.ID))
	assert.True(t, unittest.AssertExistsAndLoadBean(t, &Workflow{ID: added.ID}).Enabled, "other projects cannot modify it")

	workflows, err := db.Find[Workflow](ctx, FindWorkflowsOptions{ProjectIDs: []int64{1}})
	require.NoError(t, err)
	require.Len(t, workflows, 3)
	assert.Equal(t, []int64{closed.ID, added.ID, closed2.ID}, []int64{workflows[0].ID, workflows[1].ID, workflows[2].ID})

	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	enabled, err := FindEnabledWorkflows(ctx, repo1, []int64{1, 2}, WorkflowEventItemClosed)
	require.NoError(t, err)
	require.Len(t, enabled, 1)
	assert.Equal(t, closed2.ID, enabled[0].ID)
	enabled, err = FindEnabledWorkflows(ctx, unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2}), []int64{1}, WorkflowEventItemClosed)
	require.NoError(t, err)
	assert.Empty(t, enabled)

	require.NoError(t, ChangeProjectStatus(ctx, unittest.AssertExistsAndLoadBean(t, &Project{ID: 1}), true))
	enabled, err = FindEnabledWorkflows(ctx, repo1, []int64{1, 2}, WorkflowEventItemClosed)
	require.NoError(t, err)
	assert.Empty(t, enabled)

	require.NoError(t, DeleteWorkflow(ctx, 1, closed2.ID))
	unittest.AssertNotExistsBean(t, &Workflow{ID: closed2.ID})

	require.NoError(t, DeleteProjectByID(ctx, 1))
	unittest.AssertNotExistsBean(t, &Workflow{ProjectID: 1})
}
