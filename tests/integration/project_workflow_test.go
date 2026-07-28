// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"gitea.dev/models/db"
	project_model "gitea.dev/models/project"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/json"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// projectWorkflowFixture is the state shared by the children of
// TestProjectWorkflowsWeb. Only the owner/repo/session triple is hoisted:
// every child still creates its own distinctly titled project, so the children
// share no mutable project, column or workflow state and stay order-independent.
type projectWorkflowFixture struct {
	user    *user_model.User
	repo    *repo_model.Repository
	session *TestSession
}

func (f *projectWorkflowFixture) newProject(t *testing.T, title string) *project_model.Project {
	t.Helper()
	project := &project_model.Project{
		Title:        title,
		RepoID:       f.repo.ID,
		Type:         project_model.TypeRepository,
		TemplateType: project_model.TemplateTypeNone,
	}
	require.NoError(t, project_model.NewProject(t.Context(), project))
	return project
}

func (f *projectWorkflowFixture) newColumn(t *testing.T, project *project_model.Project, title string) *project_model.Column {
	t.Helper()
	column := &project_model.Column{Title: title, ProjectID: project.ID}
	require.NoError(t, project_model.NewColumn(t.Context(), column))
	return column
}

// workflowsLink is the web route prefix for a project's workflow endpoints.
func (f *projectWorkflowFixture) workflowsLink(project *project_model.Project) string {
	return fmt.Sprintf("/%s/%s/projects/%d/workflows", f.user.Name, f.repo.Name, project.ID)
}

func TestProjectWorkflowsWeb(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	f := &projectWorkflowFixture{
		user: unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}),
		repo: unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}),
	}
	f.session = loginUser(t, f.user.Name)

	t.Run("Page", func(t *testing.T) { testProjectWorkflowsPage(t, f) })
	t.Run("Create", func(t *testing.T) { testProjectWorkflowCreate(t, f) })
	t.Run("Update", func(t *testing.T) { testProjectWorkflowUpdate(t, f) })
	t.Run("ToggleStatus", func(t *testing.T) { testProjectWorkflowToggleStatus(t, f) })
	t.Run("Delete", func(t *testing.T) { testProjectWorkflowDelete(t, f) })
	t.Run("Permissions", func(t *testing.T) { testProjectWorkflowPermissions(t, f) })
	t.Run("Validation", func(t *testing.T) { testProjectWorkflowValidation(t, f) })
}

func testProjectWorkflowsPage(t *testing.T, f *projectWorkflowFixture) {
	project := f.newProject(t, "Test Project for Workflows")
	column1 := f.newColumn(t, project, "To Do")
	column2 := f.newColumn(t, project, "Done")

	// Create some workflows
	workflow1 := &project_model.Workflow{
		ProjectID:     project.ID,
		WorkflowEvent: project_model.WorkflowEventItemOpened,
		WorkflowFilters: []project_model.WorkflowFilter{
			{
				Type:  project_model.WorkflowFilterTypeIssueType,
				Value: "issue",
			},
		},
		WorkflowActions: []project_model.WorkflowAction{
			{
				Type:  project_model.WorkflowActionTypeColumn,
				Value: strconv.FormatInt(column1.ID, 10),
			},
		},
		Enabled: true,
	}
	err := project_model.CreateWorkflow(t.Context(), workflow1)
	assert.NoError(t, err)

	workflow2 := &project_model.Workflow{
		ProjectID:     project.ID,
		WorkflowEvent: project_model.WorkflowEventItemClosed,
		WorkflowFilters: []project_model.WorkflowFilter{
			{
				Type:  project_model.WorkflowFilterTypeIssueType,
				Value: "pull_request",
			},
		},
		WorkflowActions: []project_model.WorkflowAction{
			{
				Type:  project_model.WorkflowActionTypeColumn,
				Value: strconv.FormatInt(column2.ID, 10),
			},
		},
		Enabled: false, // Disabled workflow
	}
	err = project_model.CreateWorkflow(t.Context(), workflow2)
	assert.NoError(t, err)

	// Test accessing workflows page
	req := NewRequest(t, "GET", f.workflowsLink(project))
	resp := f.session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)

	// Verify the main workflow container exists
	assert.Positive(t, htmlDoc.Find("#project-workflows").Length(), "Main workflow container should exist")

	// Verify data attributes are set correctly
	workflowDiv := htmlDoc.Find("#project-workflows")
	assert.Positive(t, workflowDiv.Length(), "Workflow div should exist")

	// Check that locale data attributes exist
	assert.NotEmpty(t, workflowDiv.AttrOr("data-locale-default-workflows", ""), "data-locale-default-workflows should be set")
	assert.NotEmpty(t, workflowDiv.AttrOr("data-locale-when", ""), "data-locale-when should be set")
	assert.NotEmpty(t, workflowDiv.AttrOr("data-locale-actions", ""), "data-locale-actions should be set")
	assert.NotEmpty(t, workflowDiv.AttrOr("data-locale-filters", ""), "data-locale-filters should be set")
	assert.NotEmpty(t, workflowDiv.AttrOr("data-locale-close-issue", ""), "data-locale-close-issue should be set")
	assert.NotEmpty(t, workflowDiv.AttrOr("data-locale-reopen-issue", ""), "data-locale-reopen-issue should be set")
	assert.NotEmpty(t, workflowDiv.AttrOr("data-locale-issues-and-pull-requests", ""), "data-locale-issues-and-pull-requests should be set")

	// Verify project link is set
	projectLink := workflowDiv.AttrOr("data-project-link", "")
	assert.Equal(t, fmt.Sprintf("/%s/%s/projects/%d", f.user.Name, f.repo.Name, project.ID), projectLink, "Project link should be correct")
	assert.Equal(t, "true", workflowDiv.AttrOr("data-can-write-projects", ""), "owners should be able to edit workflows")

	// Test that unauthenticated users can read workflow information but cannot modify workflows.
	req = NewRequest(t, "GET", f.workflowsLink(project))
	resp = MakeRequest(t, req, http.StatusOK)
	htmlDoc = NewHTMLParser(t, resp.Body)
	workflowDiv = htmlDoc.Find("#project-workflows")
	assert.Equal(t, "false", workflowDiv.AttrOr("data-can-write-projects", ""), "readers should not see workflow edit controls")

	req = NewRequest(t, "GET", f.workflowsLink(project)+"/events")
	MakeRequest(t, req, http.StatusOK)

	req = NewRequestWithBody(t, "POST",
		fmt.Sprintf("%s/%s", f.workflowsLink(project), project_model.WorkflowEventItemOpened),
		strings.NewReader(`{"event_id":"item_opened"}`))
	req.Header.Set("Content-Type", "application/json")
	MakeRequest(t, req, http.StatusNotFound)

	// Test accessing non-existent project workflows page
	req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/projects/999999/workflows", f.user.Name, f.repo.Name))
	f.session.MakeRequest(t, req, http.StatusNotFound)

	// Verify workflows were created
	workflows, err := project_model.FindWorkflowsByProjectID(t.Context(), project.ID)
	assert.NoError(t, err)
	require.Len(t, workflows, 2, "Should have 2 workflows")

	// Verify workflow details, keyed by event rather than by position: a positional
	// assertion would still pass if FindWorkflowsByProjectID returned the rows in a
	// different order, which is exactly the kind of regression it should catch.
	byEvent := make(map[project_model.WorkflowEvent]*project_model.Workflow, len(workflows))
	for _, workflow := range workflows {
		byEvent[workflow.WorkflowEvent] = workflow
	}

	require.Contains(t, byEvent, project_model.WorkflowEventItemOpened)
	assert.True(t, byEvent[project_model.WorkflowEventItemOpened].Enabled, "the item_opened workflow should be enabled")

	require.Contains(t, byEvent, project_model.WorkflowEventItemClosed)
	assert.False(t, byEvent[project_model.WorkflowEventItemClosed].Enabled, "the item_closed workflow should be disabled")
}

func testProjectWorkflowCreate(t *testing.T, f *projectWorkflowFixture) {
	project := f.newProject(t, "Test Project for Workflow Create")
	column := f.newColumn(t, project, "Test Column")

	// Create a workflow via API
	workflowData := map[string]any{
		"event_id": string(project_model.WorkflowEventItemOpened),
		"filters": map[string]any{
			string(project_model.WorkflowFilterTypeIssueType): "issue",
		},
		"actions": map[string]any{
			string(project_model.WorkflowActionTypeColumn): strconv.FormatInt(column.ID, 10),
		},
	}

	body, err := json.Marshal(workflowData)
	assert.NoError(t, err)

	req := NewRequestWithBody(t, "POST",
		f.workflowsLink(project)+"/item_opened",
		strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	resp := f.session.MakeRequest(t, req, http.StatusOK)

	// Parse response
	var result map[string]any
	err = json.Unmarshal(resp.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.True(t, result["success"].(bool))

	// Verify workflow was created
	workflows, err := project_model.FindWorkflowsByProjectID(t.Context(), project.ID)
	assert.NoError(t, err)
	assert.Len(t, workflows, 1)
	assert.Equal(t, project_model.WorkflowEventItemOpened, workflows[0].WorkflowEvent)
	assert.True(t, workflows[0].Enabled)
}

func testProjectWorkflowUpdate(t *testing.T, f *projectWorkflowFixture) {
	project := f.newProject(t, "Test Project for Workflow Update")
	column := f.newColumn(t, project, "Test Column")

	// Create a workflow
	workflow := &project_model.Workflow{
		ProjectID:     project.ID,
		WorkflowEvent: project_model.WorkflowEventItemOpened,
		WorkflowFilters: []project_model.WorkflowFilter{
			{
				Type:  project_model.WorkflowFilterTypeIssueType,
				Value: "issue",
			},
		},
		WorkflowActions: []project_model.WorkflowAction{
			{
				Type:  project_model.WorkflowActionTypeColumn,
				Value: strconv.FormatInt(column.ID, 10),
			},
		},
		Enabled: true,
	}
	err := project_model.CreateWorkflow(t.Context(), workflow)
	assert.NoError(t, err)

	// Update the workflow
	updateData := map[string]any{
		"event_id": strconv.FormatInt(workflow.ID, 10),
		"filters": map[string]any{
			string(project_model.WorkflowFilterTypeIssueType): "pull_request", // Change to PR
		},
		"actions": map[string]any{
			string(project_model.WorkflowActionTypeColumn): strconv.FormatInt(column.ID, 10),
		},
	}

	body, err := json.Marshal(updateData)
	assert.NoError(t, err)

	req := NewRequestWithBody(t, "POST",
		fmt.Sprintf("%s/%d", f.workflowsLink(project), workflow.ID),
		strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	resp := f.session.MakeRequest(t, req, http.StatusOK)

	// Parse response
	var result map[string]any
	err = json.Unmarshal(resp.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.True(t, result["success"].(bool))

	// Verify workflow was updated
	updatedWorkflow, err := project_model.GetWorkflowByProjectAndID(t.Context(), project.ID, workflow.ID)
	assert.NoError(t, err)
	assert.True(t, updatedWorkflow.Enabled)
	assert.Len(t, updatedWorkflow.WorkflowFilters, 1)
	assert.Equal(t, "pull_request", updatedWorkflow.WorkflowFilters[0].Value)
}

func testProjectWorkflowToggleStatus(t *testing.T, f *projectWorkflowFixture) {
	project := f.newProject(t, "Test Project for Workflow Status")

	// Create a workflow that is initially enabled
	workflow := &project_model.Workflow{
		ProjectID:       project.ID,
		WorkflowEvent:   project_model.WorkflowEventItemOpened,
		WorkflowFilters: []project_model.WorkflowFilter{},
		WorkflowActions: []project_model.WorkflowAction{},
		Enabled:         true,
	}
	err := project_model.CreateWorkflow(t.Context(), workflow)
	assert.NoError(t, err)

	statusLink := fmt.Sprintf("%s/%d/status", f.workflowsLink(project), workflow.ID)

	// Test 1: Toggle status from enabled to disabled
	t.Run("Disable workflow", func(t *testing.T) {
		req := NewRequestWithValues(t, "POST", statusLink,
			map[string]string{
				"enabled": "false",
			})
		resp := f.session.MakeRequest(t, req, http.StatusOK)

		// Parse response
		var result map[string]any
		err := json.Unmarshal(resp.Body.Bytes(), &result)
		assert.NoError(t, err)
		assert.True(t, result["success"].(bool), "Response should indicate success")

		// Verify status was changed to disabled
		updatedWorkflow, err := project_model.GetWorkflowByProjectAndID(t.Context(), project.ID, workflow.ID)
		assert.NoError(t, err)
		assert.False(t, updatedWorkflow.Enabled, "Workflow should be disabled")
	})

	// Test 2: Toggle status from disabled to enabled
	t.Run("Enable workflow", func(t *testing.T) {
		req := NewRequestWithValues(t, "POST", statusLink,
			map[string]string{
				"enabled": "true",
			})
		resp := f.session.MakeRequest(t, req, http.StatusOK)

		// Parse response
		var result map[string]any
		err := json.Unmarshal(resp.Body.Bytes(), &result)
		assert.NoError(t, err)
		assert.True(t, result["success"].(bool), "Response should indicate success")

		// Verify status was changed back to enabled
		updatedWorkflow, err := project_model.GetWorkflowByProjectAndID(t.Context(), project.ID, workflow.ID)
		assert.NoError(t, err)
		assert.True(t, updatedWorkflow.Enabled, "Workflow should be enabled")
	})
}

func testProjectWorkflowDelete(t *testing.T, f *projectWorkflowFixture) {
	project := f.newProject(t, "Test Project for Workflow Delete")

	// Create a workflow
	workflow := &project_model.Workflow{
		ProjectID:       project.ID,
		WorkflowEvent:   project_model.WorkflowEventItemOpened,
		WorkflowFilters: []project_model.WorkflowFilter{},
		WorkflowActions: []project_model.WorkflowAction{},
		Enabled:         true,
	}
	err := project_model.CreateWorkflow(t.Context(), workflow)
	assert.NoError(t, err)

	deleteLink := fmt.Sprintf("%s/%d/delete", f.workflowsLink(project), workflow.ID)

	// Delete the workflow
	req := NewRequest(t, "POST", deleteLink)
	resp := f.session.MakeRequest(t, req, http.StatusOK)

	// Parse response
	var result map[string]any
	err = json.Unmarshal(resp.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.True(t, result["success"].(bool), "Delete response should indicate success")

	// Verify workflow was deleted - should return ErrNotExist
	_, err = project_model.GetWorkflowByProjectAndID(t.Context(), project.ID, workflow.ID)
	assert.Error(t, err, "Should return an error when workflow doesn't exist")
	assert.True(t, db.IsErrNotExist(err), "Error should be ErrNotExist type")

	// Verify we cannot delete it again (should fail gracefully)
	req = NewRequest(t, "POST", deleteLink)
	f.session.MakeRequest(t, req, http.StatusNotFound)
}

func testProjectWorkflowPermissions(t *testing.T, f *projectWorkflowFixture) {
	project := f.newProject(t, "Test Project for Workflow Permissions")

	// Create a workflow
	workflow := &project_model.Workflow{
		ProjectID:       project.ID,
		WorkflowEvent:   project_model.WorkflowEventItemOpened,
		WorkflowFilters: []project_model.WorkflowFilter{},
		WorkflowActions: []project_model.WorkflowAction{},
		Enabled:         true,
	}
	err := project_model.CreateWorkflow(t.Context(), workflow)
	assert.NoError(t, err)

	// User with write permission should be able to access workflows
	req := NewRequest(t, "GET", f.workflowsLink(project))
	f.session.MakeRequest(t, req, http.StatusOK)

	// User without write permission should not be able to modify workflows
	otherUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	otherSession := loginUser(t, otherUser.Name)
	req = NewRequest(t, "POST", fmt.Sprintf("%s/%d/delete", f.workflowsLink(project), workflow.ID))
	otherSession.MakeRequest(t, req, http.StatusNotFound) // we use 404 to avoid leaking existence
}

func testProjectWorkflowValidation(t *testing.T, f *projectWorkflowFixture) {
	project := f.newProject(t, "Test Project for Workflow Validation")

	// Test 1: Try to create a workflow without any actions (should fail)
	t.Run("Create workflow without actions should fail", func(t *testing.T) {
		workflowData := map[string]any{
			"event_id": string(project_model.WorkflowEventItemOpened),
			"filters": map[string]any{
				string(project_model.WorkflowFilterTypeIssueType): "issue",
			},
			"actions": map[string]any{
				// No actions provided - this should trigger validation error
			},
		}

		body, err := json.Marshal(workflowData)
		assert.NoError(t, err)

		req := NewRequestWithBody(t, "POST",
			f.workflowsLink(project)+"/item_opened",
			strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		resp := f.session.MakeRequest(t, req, http.StatusBadRequest)

		// Parse response
		var result map[string]any
		err = json.Unmarshal(resp.Body.Bytes(), &result)
		assert.NoError(t, err)
		assert.Equal(t, "At least one action must be configured", result["errorMessage"])
	})

	// Test 2: Try to update a workflow to have no actions (should fail)
	t.Run("Update workflow to remove all actions should fail", func(t *testing.T) {
		// First create a valid workflow
		column := f.newColumn(t, project, "Test Column")

		workflow := &project_model.Workflow{
			ProjectID:     project.ID,
			WorkflowEvent: project_model.WorkflowEventItemOpened,
			WorkflowFilters: []project_model.WorkflowFilter{
				{
					Type:  project_model.WorkflowFilterTypeIssueType,
					Value: "issue",
				},
			},
			WorkflowActions: []project_model.WorkflowAction{
				{
					Type:  project_model.WorkflowActionTypeColumn,
					Value: strconv.FormatInt(column.ID, 10),
				},
			},
			Enabled: true,
		}
		err := project_model.CreateWorkflow(t.Context(), workflow)
		assert.NoError(t, err)

		// Try to update it to have no actions
		updateData := map[string]any{
			"event_id": strconv.FormatInt(workflow.ID, 10),
			"filters": map[string]any{
				string(project_model.WorkflowFilterTypeIssueType): "issue",
			},
			"actions": map[string]any{
				// No actions - should fail
			},
		}

		body, err := json.Marshal(updateData)
		assert.NoError(t, err)

		req := NewRequestWithBody(t, "POST",
			fmt.Sprintf("%s/%d", f.workflowsLink(project), workflow.ID),
			strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		resp := f.session.MakeRequest(t, req, http.StatusBadRequest)

		// Parse response
		var result map[string]any
		err = json.Unmarshal(resp.Body.Bytes(), &result)
		assert.NoError(t, err)
		assert.Equal(t, "At least one action must be configured", result["errorMessage"])

		// Verify the workflow was not changed
		unchangedWorkflow, err := project_model.GetWorkflowByProjectAndID(t.Context(), project.ID, workflow.ID)
		assert.NoError(t, err)
		assert.Len(t, unchangedWorkflow.WorkflowActions, 1, "Workflow should still have the original action")
	})

	// Test 3: An unresolvable reference (e.g. a column that doesn't belong to this project) must
	// be rejected outright, not silently dropped: silently dropping it would broaden the workflow
	// to match every column instead of narrowing it to one (see convertAPIProjectWorkflowActions
	// for the equivalent, already-correct API-side policy).
	t.Run("Create workflow with an unresolvable column reference should fail", func(t *testing.T) {
		workflowsBefore, err := project_model.FindWorkflowsByProjectID(t.Context(), project.ID)
		assert.NoError(t, err)

		workflowData := map[string]any{
			"event_id": string(project_model.WorkflowEventItemOpened),
			"actions": map[string]any{
				string(project_model.WorkflowActionTypeColumn):    "999999", // no such column on this project
				string(project_model.WorkflowActionTypeAddLabels): []any{"1"},
			},
		}

		body, err := json.Marshal(workflowData)
		assert.NoError(t, err)

		req := NewRequestWithBody(t, "POST",
			f.workflowsLink(project)+"/item_opened",
			strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		resp := f.session.MakeRequest(t, req, http.StatusBadRequest)

		var result map[string]any
		err = json.Unmarshal(resp.Body.Bytes(), &result)
		assert.NoError(t, err)
		assert.Contains(t, result["errorMessage"], "column")

		workflowsAfter, err := project_model.FindWorkflowsByProjectID(t.Context(), project.ID)
		assert.NoError(t, err)
		assert.Len(t, workflowsAfter, len(workflowsBefore), "no workflow should have been created from a request with an unresolvable reference")
	})
}
