// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"fmt"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"

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
				Value: fmt.Sprintf("%d", column.ID),
			},
		},
		Enabled: true,
	}

	err = CreateWorkflow(t.Context(), workflow)
	assert.NoError(t, err)
	assert.NotZero(t, workflow.ID, "Workflow ID should be set after creation")

	// Verify the workflow was created
	createdWorkflow, err := GetWorkflowByID(t.Context(), workflow.ID)
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
	updatedWorkflow, err := GetWorkflowByID(t.Context(), workflow.ID)
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
	err = DeleteWorkflow(t.Context(), workflowID)
	assert.NoError(t, err)

	// Verify it was deleted
	_, err = GetWorkflowByID(t.Context(), workflowID)
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

	// Test Disable
	err = DisableWorkflow(t.Context(), workflow.ID)
	assert.NoError(t, err)

	disabledWorkflow, err := GetWorkflowByID(t.Context(), workflow.ID)
	assert.NoError(t, err)
	assert.False(t, disabledWorkflow.Enabled)

	// Test Enable
	err = EnableWorkflow(t.Context(), workflow.ID)
	assert.NoError(t, err)

	enabledWorkflow, err := GetWorkflowByID(t.Context(), workflow.ID)
	assert.NoError(t, err)
	assert.True(t, enabledWorkflow.Enabled)
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
	loadedWorkflow, err := GetWorkflowByID(t.Context(), workflow.ID)
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
