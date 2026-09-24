// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"strconv"
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestIsValidWorkflowEvent(t *testing.T) {
	tests := []struct {
		event string
		valid bool
	}{
		{string(WorkflowEventItemOpened), true},
		{string(WorkflowEventItemClosed), true},
		{string(WorkflowEventItemReopened), true},
		{string(WorkflowEventItemAddedToProject), true},
		{string(WorkflowEventItemRemovedFromProject), true},
		{string(WorkflowEventItemColumnChanged), true},
		{string(WorkflowEventCodeChangesRequested), true},
		{string(WorkflowEventCodeReviewApproved), true},
		{string(WorkflowEventPullRequestMerged), true},
		{"invalid_event", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.event, func(t *testing.T) {
			result := IsValidWorkflowEvent(tt.event)
			assert.Equal(t, tt.valid, result, "Event: %s", tt.event)
		})
	}
}

func TestWorkflowEventLangKey(t *testing.T) {
	tests := []struct {
		event   WorkflowEvent
		langKey string
	}{
		{WorkflowEventItemOpened, "projects.workflows.event.item_opened"},
		{WorkflowEventItemClosed, "projects.workflows.event.item_closed"},
		{WorkflowEventItemReopened, "projects.workflows.event.item_reopened"},
		{WorkflowEventItemAddedToProject, "projects.workflows.event.item_added_to_project"},
		{WorkflowEventItemRemovedFromProject, "projects.workflows.event.item_removed_from_project"},
		{WorkflowEventItemColumnChanged, "projects.workflows.event.item_column_changed"},
		{WorkflowEventCodeChangesRequested, "projects.workflows.event.code_changes_requested"},
		{WorkflowEventCodeReviewApproved, "projects.workflows.event.code_review_approved"},
		{WorkflowEventPullRequestMerged, "projects.workflows.event.pull_request_merged"},
	}

	for _, tt := range tests {
		t.Run(string(tt.event), func(t *testing.T) {
			result := tt.event.LangKey()
			assert.Equal(t, tt.langKey, result)
		})
	}
}

func TestGetWorkflowEventCapabilities(t *testing.T) {
	capabilities := GetWorkflowEventCapabilities()

	// Verify all events have capabilities
	assert.Len(t, capabilities, 9, "Should have capabilities for all 9 workflow events")

	// Test item_opened event
	itemOpenedCap := capabilities[WorkflowEventItemOpened]
	assert.Contains(t, itemOpenedCap.AvailableFilters, WorkflowFilterTypeIssueType)
	assert.Contains(t, itemOpenedCap.AvailableFilters, WorkflowFilterTypeLabels)
	assert.Contains(t, itemOpenedCap.AvailableActions, WorkflowActionTypeColumn)
	assert.Contains(t, itemOpenedCap.AvailableActions, WorkflowActionTypeAddLabels)

	// Test item_column_changed event (should have the most filters)
	columnChangedCap := capabilities[WorkflowEventItemColumnChanged]
	assert.Contains(t, columnChangedCap.AvailableFilters, WorkflowFilterTypeIssueType)
	assert.Contains(t, columnChangedCap.AvailableFilters, WorkflowFilterTypeSourceColumn)
	assert.Contains(t, columnChangedCap.AvailableFilters, WorkflowFilterTypeTargetColumn)
	assert.Contains(t, columnChangedCap.AvailableFilters, WorkflowFilterTypeLabels)

	// Test code review events (should not have issue state action)
	codeReviewCap := capabilities[WorkflowEventCodeReviewApproved]
	assert.NotContains(t, codeReviewCap.AvailableActions, WorkflowActionTypeIssueState)
}

func TestCreateWorkflow(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Get an existing project from fixtures
	project := unittest.AssertExistsAndLoadBean(t, &Project{ID: 1})

	// Create a column for the project
	column := &Column{
		Title:     "Test Column",
		ProjectID: project.ID,
	}
	err := NewColumn(t.Context(), column)
	assert.NoError(t, err)

	// Create a workflow
	workflow := &Workflow{
		ProjectID:     project.ID,
		WorkflowEvent: WorkflowEventItemOpened,
		WorkflowFilters: []WorkflowFilter{
			{
				Type:  WorkflowFilterTypeIssueType,
				Value: "issue",
			},
		},
		WorkflowActions: []WorkflowAction{
			{
				Type:  WorkflowActionTypeColumn,
				Value: strconv.FormatInt(column.ID, 10),
			},
		},
		Enabled: true,
	}

	err = CreateWorkflow(t.Context(), workflow)
	assert.NoError(t, err)
	assert.NotZero(t, workflow.ID, "Workflow ID should be set after creation")

	// Verify the workflow was created
	createdWorkflow, err := GetWorkflowByProjectAndID(t.Context(), project.ID, workflow.ID)
	assert.NoError(t, err)
	assert.Equal(t, project.ID, createdWorkflow.ProjectID)
	assert.Equal(t, WorkflowEventItemOpened, createdWorkflow.WorkflowEvent)
	assert.True(t, createdWorkflow.Enabled)
	assert.Len(t, createdWorkflow.WorkflowFilters, 1)
	assert.Len(t, createdWorkflow.WorkflowActions, 1)
	assert.Equal(t, WorkflowFilterTypeIssueType, createdWorkflow.WorkflowFilters[0].Type)
	assert.Equal(t, "issue", createdWorkflow.WorkflowFilters[0].Value)
	assert.Equal(t, WorkflowActionTypeColumn, createdWorkflow.WorkflowActions[0].Type)
}

func TestUpdateWorkflow(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Get an existing project from fixtures
	project := unittest.AssertExistsAndLoadBean(t, &Project{ID: 1})

	// Create a workflow
	workflow := &Workflow{
		ProjectID:       project.ID,
		WorkflowEvent:   WorkflowEventItemOpened,
		WorkflowFilters: []WorkflowFilter{},
		WorkflowActions: []WorkflowAction{},
		Enabled:         true,
	}
	err := CreateWorkflow(t.Context(), workflow)
	assert.NoError(t, err)

	// Update the workflow
	workflow.WorkflowFilters = []WorkflowFilter{
		{
			Type:  WorkflowFilterTypeIssueType,
			Value: "pull_request",
		},
	}

	err = UpdateWorkflow(t.Context(), workflow)
	assert.NoError(t, err)

	// Verify the update
	updatedWorkflow, err := GetWorkflowByProjectAndID(t.Context(), project.ID, workflow.ID)
	assert.NoError(t, err)
	assert.True(t, updatedWorkflow.Enabled)
	assert.Len(t, updatedWorkflow.WorkflowFilters, 1)
	assert.Equal(t, "pull_request", updatedWorkflow.WorkflowFilters[0].Value)
}

func TestDeleteWorkflow(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Get an existing project from fixtures
	project := unittest.AssertExistsAndLoadBean(t, &Project{ID: 1})

	// Create a workflow
	workflow := &Workflow{
		ProjectID:       project.ID,
		WorkflowEvent:   WorkflowEventItemOpened,
		WorkflowFilters: []WorkflowFilter{},
		WorkflowActions: []WorkflowAction{},
		Enabled:         true,
	}
	err := CreateWorkflow(t.Context(), workflow)
	assert.NoError(t, err)

	workflowID := workflow.ID

	// Delete the workflow
	// a foreign project ID must not delete the workflow
	err = DeleteWorkflow(t.Context(), project.ID+1, workflowID)
	assert.NoError(t, err)
	_, err = GetWorkflowByProjectAndID(t.Context(), project.ID, workflowID)
	assert.NoError(t, err)

	err = DeleteWorkflow(t.Context(), project.ID, workflowID)
	assert.NoError(t, err)

	// Verify it was deleted
	_, err = GetWorkflowByProjectAndID(t.Context(), project.ID, workflowID)
	assert.Error(t, err)
	assert.True(t, db.IsErrNotExist(err), "Should return ErrNotExist")
}

func TestEnableDisableWorkflow(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Get an existing project from fixtures
	project := unittest.AssertExistsAndLoadBean(t, &Project{ID: 1})

	// Create a workflow (enabled by default)
	workflow := &Workflow{
		ProjectID:       project.ID,
		WorkflowEvent:   WorkflowEventItemOpened,
		WorkflowFilters: []WorkflowFilter{},
		WorkflowActions: []WorkflowAction{},
		Enabled:         true,
	}
	err := CreateWorkflow(t.Context(), workflow)
	assert.NoError(t, err)

	// a foreign project ID must not touch the workflow
	err = DisableWorkflow(t.Context(), project.ID+1, workflow.ID)
	assert.NoError(t, err)
	untouchedWorkflow, err := GetWorkflowByProjectAndID(t.Context(), project.ID, workflow.ID)
	assert.NoError(t, err)
	assert.True(t, untouchedWorkflow.Enabled)

	// Test Disable
	err = DisableWorkflow(t.Context(), project.ID, workflow.ID)
	assert.NoError(t, err)

	disabledWorkflow, err := GetWorkflowByProjectAndID(t.Context(), project.ID, workflow.ID)
	assert.NoError(t, err)
	assert.False(t, disabledWorkflow.Enabled)

	// Test Enable
	err = EnableWorkflow(t.Context(), project.ID, workflow.ID)
	assert.NoError(t, err)

	enabledWorkflow, err := GetWorkflowByProjectAndID(t.Context(), project.ID, workflow.ID)
	assert.NoError(t, err)
	assert.True(t, enabledWorkflow.Enabled)
}

func TestCreateWorkflowDefaultsSchemaVersion(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	project := unittest.AssertExistsAndLoadBean(t, &Project{ID: 1})

	// callers that don't set SchemaVersion (i.e. every caller today) must still
	// get CurrentWorkflowSchemaVersion persisted, not the Go zero value: xorm's
	// Insert sends the zero value literally, so the "DEFAULT 1" column default
	// is never applied for rows inserted through CreateWorkflow.
	workflow := &Workflow{
		ProjectID:       project.ID,
		WorkflowEvent:   WorkflowEventItemOpened,
		WorkflowFilters: []WorkflowFilter{},
		WorkflowActions: []WorkflowAction{},
		Enabled:         true,
	}
	assert.NoError(t, CreateWorkflow(t.Context(), workflow))
	assert.Equal(t, CurrentWorkflowSchemaVersion, workflow.SchemaVersion)

	loadedWorkflow, err := GetWorkflowByProjectAndID(t.Context(), project.ID, workflow.ID)
	assert.NoError(t, err)
	assert.Equal(t, CurrentWorkflowSchemaVersion, loadedWorkflow.SchemaVersion)

	// an explicitly set SchemaVersion (e.g. a future v2 caller) must be preserved as-is
	explicitWorkflow := &Workflow{
		ProjectID:       project.ID,
		WorkflowEvent:   WorkflowEventItemClosed,
		WorkflowFilters: []WorkflowFilter{},
		WorkflowActions: []WorkflowAction{},
		Enabled:         true,
		SchemaVersion:   2,
	}
	assert.NoError(t, CreateWorkflow(t.Context(), explicitWorkflow))
	assert.Equal(t, 2, explicitWorkflow.SchemaVersion)
}

func TestFindWorkflowsByProjectID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Get an existing project from fixtures
	project := unittest.AssertExistsAndLoadBean(t, &Project{ID: 1})

	// Create multiple workflows
	workflow1 := &Workflow{
		ProjectID:       project.ID,
		WorkflowEvent:   WorkflowEventItemOpened,
		WorkflowFilters: []WorkflowFilter{},
		WorkflowActions: []WorkflowAction{},
		Enabled:         true,
	}
	err := CreateWorkflow(t.Context(), workflow1)
	assert.NoError(t, err)

	workflow2 := &Workflow{
		ProjectID:       project.ID,
		WorkflowEvent:   WorkflowEventItemClosed,
		WorkflowFilters: []WorkflowFilter{},
		WorkflowActions: []WorkflowAction{},
		Enabled:         false,
	}
	err = CreateWorkflow(t.Context(), workflow2)
	assert.NoError(t, err)

	// Find all workflows for the project
	workflows, err := FindWorkflowsByProjectID(t.Context(), project.ID)
	assert.NoError(t, err)
	assert.Len(t, workflows, 2)

	// Verify the workflows
	assert.Equal(t, WorkflowEventItemOpened, workflows[0].WorkflowEvent)
	assert.True(t, workflows[0].Enabled)
	assert.Equal(t, WorkflowEventItemClosed, workflows[1].WorkflowEvent)
	assert.False(t, workflows[1].Enabled)
}

// TestFindWorkflowsByProjectIDOrderedByID locks in the explicit ORDER BY id ASC:
// without it, row order is engine-dependent (e.g. an UPDATE can relocate a row
// on Postgres), so toggling Enabled could otherwise silently change the order
// in which workflows execute for the same event.
func TestFindWorkflowsByProjectIDOrderedByID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	project := unittest.AssertExistsAndLoadBean(t, &Project{ID: 1})

	var ids []int64
	for range 3 {
		wf := &Workflow{
			ProjectID:       project.ID,
			WorkflowEvent:   WorkflowEventItemOpened,
			WorkflowFilters: []WorkflowFilter{},
			WorkflowActions: []WorkflowAction{},
			Enabled:         true,
		}
		assert.NoError(t, CreateWorkflow(t.Context(), wf))
		ids = append(ids, wf.ID)
	}

	// toggling Enabled on the first-created workflow must not change its position
	assert.NoError(t, DisableWorkflow(t.Context(), project.ID, ids[0]))
	assert.NoError(t, EnableWorkflow(t.Context(), project.ID, ids[0]))

	workflows, err := FindWorkflowsByProjectID(t.Context(), project.ID)
	assert.NoError(t, err)
	if assert.Len(t, workflows, 3) {
		for i, wf := range workflows {
			assert.Equal(t, ids[i], wf.ID)
		}
	}
}

func TestWorkflowLoadProject(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Get an existing project from fixtures
	project := unittest.AssertExistsAndLoadBean(t, &Project{ID: 1})

	// Create a workflow
	workflow := &Workflow{
		ProjectID:       project.ID,
		WorkflowEvent:   WorkflowEventItemOpened,
		WorkflowFilters: []WorkflowFilter{},
		WorkflowActions: []WorkflowAction{},
		Enabled:         true,
	}
	err := CreateWorkflow(t.Context(), workflow)
	assert.NoError(t, err)

	// Get the workflow
	loadedWorkflow, err := GetWorkflowByProjectAndID(t.Context(), project.ID, workflow.ID)
	assert.NoError(t, err)
	assert.Nil(t, loadedWorkflow.Project)

	// Load the project
	err = loadedWorkflow.LoadProject(t.Context())
	assert.NoError(t, err)
	assert.NotNil(t, loadedWorkflow.Project)
	assert.Equal(t, project.ID, loadedWorkflow.Project.ID)

	// Load again should not error
	err = loadedWorkflow.LoadProject(t.Context())
	assert.NoError(t, err)
}
