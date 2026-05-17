// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	issues_model "code.gitea.io/gitea/models/issues"
	project_model "code.gitea.io/gitea/models/project"
	"code.gitea.io/gitea/modules/json"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIRepoProjectWorkflows(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	project := &project_model.Project{
		Title:        "API project workflows",
		RepoID:       1,
		CreatorID:    2,
		Type:         project_model.TypeRepository,
		TemplateType: project_model.TemplateTypeNone,
	}
	require.NoError(t, project_model.NewProject(t.Context(), project))

	column := &project_model.Column{Title: "API Column", ProjectID: project.ID}
	require.NoError(t, project_model.NewColumn(t.Context(), column))

	label := &issues_model.Label{RepoID: 1, Name: "api-workflow", Color: "0055ff"}
	require.NoError(t, issues_model.NewLabel(t.Context(), label))

	ownerToken := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteRepository)
	readerToken := getUserToken(t, "user1", auth_model.AccessTokenScopeReadRepository)

	listURL := fmt.Sprintf("/api/v1/repos/user2/repo1/projects/%d/workflows", project.ID)
	optionsURL := listURL + "/options"

	t.Run("get options", func(t *testing.T) {
		resp := MakeRequest(t, NewRequest(t, "GET", optionsURL).AddTokenAuth(readerToken), http.StatusOK)
		var options api.ProjectWorkflowOptions
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &options))
		assert.Contains(t, options.Columns, &api.ProjectWorkflowColumnOption{ID: column.ID, Title: column.Title, Color: column.Color})
		assert.Contains(t, options.Labels, &api.Label{ID: label.ID, Name: label.Name, Color: label.Color, Description: label.Description, Exclusive: label.Exclusive, ExclusiveOrder: label.ExclusiveOrder})
	})

	var workflow api.ProjectWorkflow
	t.Run("create workflow", func(t *testing.T) {
		resp := MakeRequest(t, NewRequestWithJSON(t, "POST", listURL, &api.CreateProjectWorkflowOption{
			EventID: string(project_model.WorkflowEventItemOpened),
			Filters: api.ProjectWorkflowFilterOptions{IssueType: "issue"},
			Actions: api.ProjectWorkflowActionOptions{Column: strconv.FormatInt(column.ID, 10)},
		}).AddTokenAuth(ownerToken), http.StatusCreated)
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &workflow))
		assert.Equal(t, project_model.WorkflowEventItemOpened.EventID(), workflow.EventID)
		assert.True(t, workflow.IsConfigured)
		assert.True(t, workflow.Enabled)
		assert.NotZero(t, workflow.ID)
	})

	t.Run("reader cannot create workflow", func(t *testing.T) {
		MakeRequest(t, NewRequestWithJSON(t, "POST", listURL, &api.CreateProjectWorkflowOption{
			EventID: string(project_model.WorkflowEventItemClosed),
			Actions: api.ProjectWorkflowActionOptions{Column: strconv.FormatInt(column.ID, 10)},
		}).AddTokenAuth(readerToken), http.StatusForbidden)
	})

	t.Run("list workflows", func(t *testing.T) {
		resp := MakeRequest(t, NewRequest(t, "GET", listURL).AddTokenAuth(readerToken), http.StatusOK)
		var workflows []api.ProjectWorkflow
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &workflows))

		foundConfigured := false
		foundPlaceholder := false
		for _, entry := range workflows {
			if entry.ID == workflow.ID {
				foundConfigured = true
			}
			if entry.ID == 0 && entry.EventID == string(project_model.WorkflowEventItemClosed) {
				foundPlaceholder = true
			}
		}
		assert.True(t, foundConfigured)
		assert.True(t, foundPlaceholder)
	})

	t.Run("get workflow", func(t *testing.T) {
		resp := MakeRequest(t, NewRequestf(t, "GET", "%s/%d", listURL, workflow.ID).AddTokenAuth(readerToken), http.StatusOK)
		var fetched api.ProjectWorkflow
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &fetched))
		assert.Equal(t, workflow.ID, fetched.ID)
		assert.Equal(t, workflow.EventID, fetched.EventID)
	})

	t.Run("update workflow", func(t *testing.T) {
		resp := MakeRequest(t, NewRequestWithJSON(t, "PATCH", fmt.Sprintf("%s/%d", listURL, workflow.ID), &api.EditProjectWorkflowOption{
			Filters: api.ProjectWorkflowFilterOptions{IssueType: "issue", Labels: []string{strconv.FormatInt(label.ID, 10)}},
			Actions: api.ProjectWorkflowActionOptions{AddLabels: []string{strconv.FormatInt(label.ID, 10)}, IssueState: "close"},
		}).AddTokenAuth(ownerToken), http.StatusOK)

		var updated api.ProjectWorkflow
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &updated))
		assert.Equal(t, workflow.ID, updated.ID)
		assert.NotEmpty(t, updated.Actions)
		assert.NotEmpty(t, updated.Filters)
	})

	t.Run("disable and enable workflow", func(t *testing.T) {
		MakeRequest(t, NewRequestf(t, "PUT", "%s/%d/disable", listURL, workflow.ID).AddTokenAuth(ownerToken), http.StatusNoContent)

		resp := MakeRequest(t, NewRequestf(t, "GET", "%s/%d", listURL, workflow.ID).AddTokenAuth(readerToken), http.StatusOK)
		var disabled api.ProjectWorkflow
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &disabled))
		assert.False(t, disabled.Enabled)

		MakeRequest(t, NewRequestf(t, "PUT", "%s/%d/enable", listURL, workflow.ID).AddTokenAuth(ownerToken), http.StatusNoContent)

		resp = MakeRequest(t, NewRequestf(t, "GET", "%s/%d", listURL, workflow.ID).AddTokenAuth(readerToken), http.StatusOK)
		var enabled api.ProjectWorkflow
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &enabled))
		assert.True(t, enabled.Enabled)
	})

	t.Run("delete workflow", func(t *testing.T) {
		MakeRequest(t, NewRequestf(t, "DELETE", "%s/%d", listURL, workflow.ID).AddTokenAuth(ownerToken), http.StatusNoContent)
		MakeRequest(t, NewRequestf(t, "GET", "%s/%d", listURL, workflow.ID).AddTokenAuth(readerToken), http.StatusNotFound)
	})
}
