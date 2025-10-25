// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/db"
	project_model "code.gitea.io/gitea/models/project"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestProjectWorkflowsPage(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	// Create a project
	project := &project_model.Project{
		Title:        "Test Project for Workflows",
		RepoID:       repo.ID,
		Type:         project_model.TypeRepository,
		TemplateType: project_model.TemplateTypeNone,
	}
	err := project_model.NewProject(t.Context(), project)
	assert.NoError(t, err)

	// Create columns for the project
	column1 := &project_model.Column{
		Title:     "To Do",
		ProjectID: project.ID,
	}
	err = project_model.NewColumn(t.Context(), column1)
	assert.NoError(t, err)

	column2 := &project_model.Column{
		Title:     "Done",
		ProjectID: project.ID,
	}
	err = project_model.NewColumn(t.Context(), column2)
	assert.NoError(t, err)

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
				Value: fmt.Sprintf("%d", column1.ID),
			},
		},
		Enabled: true,
	}
	err = project_model.CreateWorkflow(t.Context(), workflow1)
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
				Value: fmt.Sprintf("%d", column2.ID),
			},
		},
		Enabled: false, // Disabled workflow
	}
	err = project_model.CreateWorkflow(t.Context(), workflow2)
	assert.NoError(t, err)

	session := loginUser(t, user.Name)

	// Test accessing workflows page
	req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s/projects/%d/workflows", user.Name, repo.Name, project.ID))
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)

	// Verify the main workflow container exists
	assert.True(t, htmlDoc.Find("#project-workflows").Length() > 0, "Main workflow container should exist")

	// Verify data attributes are set correctly
	workflowDiv := htmlDoc.Find("#project-workflows")
	assert.True(t, workflowDiv.Length() > 0, "Workflow div should exist")

	// Check that locale data attributes exist
	assert.True(t, workflowDiv.AttrOr("data-locale-default-workflows", "") != "", "data-locale-default-workflows should be set")
	assert.True(t, workflowDiv.AttrOr("data-locale-when", "") != "", "data-locale-when should be set")
	assert.True(t, workflowDiv.AttrOr("data-locale-actions", "") != "", "data-locale-actions should be set")
	assert.True(t, workflowDiv.AttrOr("data-locale-filters", "") != "", "data-locale-filters should be set")
	assert.True(t, workflowDiv.AttrOr("data-locale-close-issue", "") != "", "data-locale-close-issue should be set")
	assert.True(t, workflowDiv.AttrOr("data-locale-reopen-issue", "") != "", "data-locale-reopen-issue should be set")
	assert.True(t, workflowDiv.AttrOr("data-locale-issues-and-pull-requests", "") != "", "data-locale-issues-and-pull-requests should be set")

	// Verify project link is set
	projectLink := workflowDiv.AttrOr("data-project-link", "")
	assert.Equal(t, fmt.Sprintf("/%s/%s/projects/%d", user.Name, repo.Name, project.ID), projectLink, "Project link should be correct")

	// Test that unauthenticated users cannot access
	req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/projects/%d/workflows", user.Name, repo.Name, project.ID))
	MakeRequest(t, req, http.StatusNotFound)

	// Test accessing non-existent project workflows page
	req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/projects/999999/workflows", user.Name, repo.Name))
	session.MakeRequest(t, req, http.StatusNotFound)

	// Verify workflows were created
	workflows, err := project_model.FindWorkflowsByProjectID(t.Context(), project.ID)
	assert.NoError(t, err)
	assert.Len(t, workflows, 2, "Should have 2 workflows")

	// Verify workflow details
	assert.Equal(t, project_model.WorkflowEventItemOpened, workflows[0].WorkflowEvent)
	assert.True(t, workflows[0].Enabled, "First workflow should be enabled")
	assert.Equal(t, project_model.WorkflowEventItemClosed, workflows[1].WorkflowEvent)
	assert.False(t, workflows[1].Enabled, "Second workflow should be disabled")
}

func TestProjectWorkflowCreate(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	// Create a project
	project := &project_model.Project{
		Title:        "Test Project for Workflow Create",
		RepoID:       repo.ID,
		Type:         project_model.TypeRepository,
		TemplateType: project_model.TemplateTypeNone,
	}
	err := project_model.NewProject(t.Context(), project)
	assert.NoError(t, err)

	// Create a column
	column := &project_model.Column{
		Title:     "Test Column",
		ProjectID: project.ID,
	}
	err = project_model.NewColumn(t.Context(), column)
	assert.NoError(t, err)

	session := loginUser(t, user.Name)

	// Create a workflow via API
	workflowData := map[string]any{
		"event_id": string(project_model.WorkflowEventItemOpened),
		"filters": map[string]any{
			string(project_model.WorkflowFilterTypeIssueType): "issue",
		},
		"actions": map[string]any{
			string(project_model.WorkflowActionTypeColumn): fmt.Sprintf("%d", column.ID),
		},
	}

	body, err := json.Marshal(workflowData)
	assert.NoError(t, err)

	req := NewRequestWithBody(t, "POST",
		fmt.Sprintf("/%s/%s/projects/%d/workflows/item_opened?_csrf=%s", user.Name, repo.Name, project.ID, GetUserCSRFToken(t, session)),
		strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	resp := session.MakeRequest(t, req, http.StatusOK)

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

func TestProjectWorkflowUpdate(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	// Create a project
	project := &project_model.Project{
		Title:        "Test Project for Workflow Update",
		RepoID:       repo.ID,
		Type:         project_model.TypeRepository,
		TemplateType: project_model.TemplateTypeNone,
	}
	err := project_model.NewProject(t.Context(), project)
	assert.NoError(t, err)

	// Create a column
	column := &project_model.Column{
		Title:     "Test Column",
		ProjectID: project.ID,
	}
	err = project_model.NewColumn(t.Context(), column)
	assert.NoError(t, err)

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
				Value: fmt.Sprintf("%d", column.ID),
			},
		},
		Enabled: true,
	}
	err = project_model.CreateWorkflow(t.Context(), workflow)
	assert.NoError(t, err)

	session := loginUser(t, user.Name)

	// Update the workflow
	updateData := map[string]any{
		"event_id": fmt.Sprintf("%d", workflow.ID),
		"filters": map[string]any{
			string(project_model.WorkflowFilterTypeIssueType): "pull_request", // Change to PR
		},
		"actions": map[string]any{
			string(project_model.WorkflowActionTypeColumn): fmt.Sprintf("%d", column.ID),
		},
	}

	body, err := json.Marshal(updateData)
	assert.NoError(t, err)

	req := NewRequestWithBody(t, "POST",
		fmt.Sprintf("/%s/%s/projects/%d/workflows/%d?_csrf=%s", user.Name, repo.Name, project.ID, workflow.ID, GetUserCSRFToken(t, session)),
		strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	resp := session.MakeRequest(t, req, http.StatusOK)

	// Parse response
	var result map[string]any
	err = json.Unmarshal(resp.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.True(t, result["success"].(bool))

	// Verify workflow was updated
	updatedWorkflow, err := project_model.GetWorkflowByID(t.Context(), workflow.ID)
	assert.NoError(t, err)
	assert.True(t, updatedWorkflow.Enabled)
	assert.Len(t, updatedWorkflow.WorkflowFilters, 1)
	assert.Equal(t, "pull_request", updatedWorkflow.WorkflowFilters[0].Value)
}

func TestProjectWorkflowToggleStatus(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	// Create a project
	project := &project_model.Project{
		Title:        "Test Project for Workflow Status",
		RepoID:       repo.ID,
		Type:         project_model.TypeRepository,
		TemplateType: project_model.TemplateTypeNone,
	}
	err := project_model.NewProject(t.Context(), project)
	assert.NoError(t, err)

	// Create a workflow that is initially enabled
	workflow := &project_model.Workflow{
		ProjectID:       project.ID,
		WorkflowEvent:   project_model.WorkflowEventItemOpened,
		WorkflowFilters: []project_model.WorkflowFilter{},
		WorkflowActions: []project_model.WorkflowAction{},
		Enabled:         true,
	}
	err = project_model.CreateWorkflow(t.Context(), workflow)
	assert.NoError(t, err)

	session := loginUser(t, user.Name)

	// Test 1: Toggle status from enabled to disabled
	t.Run("Disable workflow", func(t *testing.T) {
		req := NewRequestWithValues(t, "POST",
			fmt.Sprintf("/%s/%s/projects/%d/workflows/%d/status?_csrf=%s", user.Name, repo.Name, project.ID, workflow.ID, GetUserCSRFToken(t, session)),
			map[string]string{
				"enabled": "false",
			})
		resp := session.MakeRequest(t, req, http.StatusOK)

		// Parse response
		var result map[string]any
		err = json.Unmarshal(resp.Body.Bytes(), &result)
		assert.NoError(t, err)
		assert.True(t, result["success"].(bool), "Response should indicate success")

		// Verify status was changed to disabled
		updatedWorkflow, err := project_model.GetWorkflowByID(t.Context(), workflow.ID)
		assert.NoError(t, err)
		assert.False(t, updatedWorkflow.Enabled, "Workflow should be disabled")
	})

	// Test 2: Toggle status from disabled to enabled
	t.Run("Enable workflow", func(t *testing.T) {
		req := NewRequestWithValues(t, "POST",
			fmt.Sprintf("/%s/%s/projects/%d/workflows/%d/status?_csrf=%s", user.Name, repo.Name, project.ID, workflow.ID, GetUserCSRFToken(t, session)),
			map[string]string{
				"enabled": "true",
			})
		resp := session.MakeRequest(t, req, http.StatusOK)

		// Parse response
		var result map[string]any
		err = json.Unmarshal(resp.Body.Bytes(), &result)
		assert.NoError(t, err)
		assert.True(t, result["success"].(bool), "Response should indicate success")

		// Verify status was changed back to enabled
		updatedWorkflow, err := project_model.GetWorkflowByID(t.Context(), workflow.ID)
		assert.NoError(t, err)
		assert.True(t, updatedWorkflow.Enabled, "Workflow should be enabled")
	})
}

func TestProjectWorkflowDelete(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	// Create a project
	project := &project_model.Project{
		Title:        "Test Project for Workflow Delete",
		RepoID:       repo.ID,
		Type:         project_model.TypeRepository,
		TemplateType: project_model.TemplateTypeNone,
	}
	err := project_model.NewProject(t.Context(), project)
	assert.NoError(t, err)

	// Create a workflow
	workflow := &project_model.Workflow{
		ProjectID:       project.ID,
		WorkflowEvent:   project_model.WorkflowEventItemOpened,
		WorkflowFilters: []project_model.WorkflowFilter{},
		WorkflowActions: []project_model.WorkflowAction{},
		Enabled:         true,
	}
	err = project_model.CreateWorkflow(t.Context(), workflow)
	assert.NoError(t, err)

	session := loginUser(t, user.Name)

	// Delete the workflow
	req := NewRequest(t, "POST",
		fmt.Sprintf("/%s/%s/projects/%d/workflows/%d/delete?_csrf=%s", user.Name, repo.Name, project.ID, workflow.ID, GetUserCSRFToken(t, session)))
	resp := session.MakeRequest(t, req, http.StatusOK)

	// Parse response
	var result map[string]any
	err = json.Unmarshal(resp.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.True(t, result["success"].(bool), "Delete response should indicate success")

	// Verify workflow was deleted - should return ErrNotExist
	_, err = project_model.GetWorkflowByID(t.Context(), workflow.ID)
	assert.Error(t, err, "Should return an error when workflow doesn't exist")
	assert.True(t, db.IsErrNotExist(err), "Error should be ErrNotExist type")

	// Verify we cannot delete it again (should fail gracefully)
	req = NewRequest(t, "POST",
		fmt.Sprintf("/%s/%s/projects/%d/workflows/%d/delete?_csrf=%s", user.Name, repo.Name, project.ID, workflow.ID, GetUserCSRFToken(t, session)))
	resp = session.MakeRequest(t, req, http.StatusNotFound)
}

func TestProjectWorkflowPermissions(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	// Create a project
	project := &project_model.Project{
		Title:        "Test Project for Workflow Permissions",
		RepoID:       repo.ID,
		Type:         project_model.TypeRepository,
		TemplateType: project_model.TemplateTypeNone,
	}
	err := project_model.NewProject(t.Context(), project)
	assert.NoError(t, err)

	// Create a workflow
	workflow := &project_model.Workflow{
		ProjectID:       project.ID,
		WorkflowEvent:   project_model.WorkflowEventItemOpened,
		WorkflowFilters: []project_model.WorkflowFilter{},
		WorkflowActions: []project_model.WorkflowAction{},
		Enabled:         true,
	}
	err = project_model.CreateWorkflow(t.Context(), workflow)
	assert.NoError(t, err)

	// User with write permission should be able to access workflows
	session1 := loginUser(t, user.Name)
	req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s/projects/%d/workflows", user.Name, repo.Name, project.ID))
	session1.MakeRequest(t, req, http.StatusOK)

	// User without write permission should not be able to modify workflows
	session2 := loginUser(t, user2.Name)
	req = NewRequest(t, "POST",
		fmt.Sprintf("/%s/%s/projects/%d/workflows/%d/delete?_csrf=%s", user.Name, repo.Name, project.ID, workflow.ID, GetUserCSRFToken(t, session2)))
	session2.MakeRequest(t, req, http.StatusForbidden)
}
