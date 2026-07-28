// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"

	auth_model "gitea.dev/models/auth"
	issues_model "gitea.dev/models/issues"
	project_model "gitea.dev/models/project"
	"gitea.dev/modules/json"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

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

	t.Run("create workflow rejects unresolvable column reference", func(t *testing.T) {
		resp := MakeRequest(t, NewRequestWithJSON(t, "POST", listURL, &api.CreateProjectWorkflowOption{
			EventID: string(project_model.WorkflowEventItemOpened),
			Actions: api.ProjectWorkflowActionOptions{Column: "999999"},
		}).AddTokenAuth(ownerToken), http.StatusUnprocessableEntity)
		assert.Contains(t, resp.Body.String(), "invalid column")
	})

	t.Run("create workflow rejects unresolvable label reference", func(t *testing.T) {
		resp := MakeRequest(t, NewRequestWithJSON(t, "POST", listURL, &api.CreateProjectWorkflowOption{
			EventID: string(project_model.WorkflowEventItemOpened),
			Actions: api.ProjectWorkflowActionOptions{AddLabels: []string{"999999"}},
		}).AddTokenAuth(ownerToken), http.StatusUnprocessableEntity)
		assert.Contains(t, resp.Body.String(), "invalid label")
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
			Filters: &api.ProjectWorkflowFilterOptions{IssueType: "issue", Labels: []string{strconv.FormatInt(label.ID, 10)}},
			Actions: &api.ProjectWorkflowActionOptions{AddLabels: []string{strconv.FormatInt(label.ID, 10)}, IssueState: "close"},
		}).AddTokenAuth(ownerToken), http.StatusOK)

		var updated api.ProjectWorkflow
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &updated))
		assert.Equal(t, workflow.ID, updated.ID)
		assert.NotEmpty(t, updated.Actions)
		assert.NotEmpty(t, updated.Filters)
	})

	t.Run("update workflow with only actions leaves filters untouched", func(t *testing.T) {
		resp := MakeRequest(t, NewRequestWithJSON(t, "PATCH", fmt.Sprintf("%s/%d", listURL, workflow.ID), &api.EditProjectWorkflowOption{
			Actions: &api.ProjectWorkflowActionOptions{AddLabels: []string{strconv.FormatInt(label.ID, 10)}},
		}).AddTokenAuth(ownerToken), http.StatusOK)

		var updated api.ProjectWorkflow
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &updated))
		assert.Equal(t, workflow.ID, updated.ID)
		assert.NotEmpty(t, updated.Filters, "omitted filters must survive a partial PATCH")
		assert.NotEmpty(t, updated.Actions)
	})

	t.Run("update workflow rejects unresolvable label reference", func(t *testing.T) {
		resp := MakeRequest(t, NewRequestWithJSON(t, "PATCH", fmt.Sprintf("%s/%d", listURL, workflow.ID), &api.EditProjectWorkflowOption{
			Filters: &api.ProjectWorkflowFilterOptions{Labels: []string{"999999"}},
		}).AddTokenAuth(ownerToken), http.StatusUnprocessableEntity)
		assert.Contains(t, resp.Body.String(), "invalid label")
	})

	t.Run("update workflow rejects unresolvable column reference", func(t *testing.T) {
		resp := MakeRequest(t, NewRequestWithJSON(t, "POST", listURL, &api.CreateProjectWorkflowOption{
			EventID: string(project_model.WorkflowEventItemColumnChanged),
			Actions: api.ProjectWorkflowActionOptions{AddLabels: []string{strconv.FormatInt(label.ID, 10)}},
		}).AddTokenAuth(ownerToken), http.StatusCreated)
		var columnChanged api.ProjectWorkflow
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &columnChanged))

		resp = MakeRequest(t, NewRequestWithJSON(t, "PATCH", fmt.Sprintf("%s/%d", listURL, columnChanged.ID), &api.EditProjectWorkflowOption{
			Filters: &api.ProjectWorkflowFilterOptions{SourceColumn: "not-a-number"},
		}).AddTokenAuth(ownerToken), http.StatusUnprocessableEntity)
		assert.Contains(t, resp.Body.String(), "invalid source_column")
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
