// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	project_model "gitea.dev/models/project"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIProjectWorkflows(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	token := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteIssue, auth_model.AccessTokenScopeWriteOrganization)
	listURL := "/api/v1/repos/user2/repo1/projects/1/workflows"

	req := NewRequestWithJSON(t, "POST", listURL, &api.CreateProjectWorkflowOption{
		Event:   "item_column_changed",
		Filters: api.ProjectWorkflowFilters{IssueType: "issue", TargetColumnID: 3},
		Actions: api.ProjectWorkflowActions{AddLabelIDs: []int64{1}, IssueState: "close"},
	}).AddTokenAuth(token)
	created := DecodeJSON(t, MakeRequest(t, req, http.StatusCreated), &api.ProjectWorkflow{})
	assert.Equal(t, "item_column_changed", created.Event)
	assert.True(t, created.Enabled)
	assert.Equal(t, api.ProjectWorkflowFilters{IssueType: "issue", TargetColumnID: 3}, created.Filters)
	assert.Equal(t, api.ProjectWorkflowActions{AddLabelIDs: []int64{1}, IssueState: "close"}, created.Actions)
	workflowURL := fmt.Sprintf("%s/%d", listURL, created.ID)

	fetched := DecodeJSON(t, MakeRequest(t, NewRequest(t, "GET", workflowURL).AddTokenAuth(token), http.StatusOK), &api.ProjectWorkflow{})
	assert.Equal(t, created.Filters, fetched.Filters)
	assert.Equal(t, created.Actions, fetched.Actions)

	resp := MakeRequest(t, NewRequest(t, "GET", listURL).AddTokenAuth(token), http.StatusOK)
	assert.Equal(t, "1", resp.Header().Get("X-Total-Count"))
	listed := DecodeJSON(t, resp, []*api.ProjectWorkflow{})
	require.Len(t, listed, 1)
	assert.Equal(t, created.ID, listed[0].ID)

	req = NewRequestWithJSON(t, "PATCH", workflowURL, &api.EditProjectWorkflowOption{Filters: &api.ProjectWorkflowFilters{SourceColumnID: 1}}).AddTokenAuth(token)
	updated := DecodeJSON(t, MakeRequest(t, req, http.StatusOK), &api.ProjectWorkflow{})
	assert.Equal(t, api.ProjectWorkflowFilters{SourceColumnID: 1}, updated.Filters)
	assert.Equal(t, created.Actions, updated.Actions)

	req = NewRequestWithJSON(t, "PATCH", workflowURL, &api.EditProjectWorkflowOption{Filters: &api.ProjectWorkflowFilters{LabelIDs: []int64{999999}}}).AddTokenAuth(token)
	assert.Contains(t, MakeRequest(t, req, http.StatusUnprocessableEntity).Body.String(), "invalid label")

	outsider := getUserToken(t, "user4", auth_model.AccessTokenScopeWriteIssue)
	req = NewRequestWithJSON(t, "POST", listURL, &api.CreateProjectWorkflowOption{Event: "item_added_to_project", Actions: api.ProjectWorkflowActions{ColumnID: 3}}).AddTokenAuth(outsider)
	MakeRequest(t, req, http.StatusForbidden)
	MakeRequest(t, NewRequest(t, "DELETE", workflowURL).AddTokenAuth(outsider), http.StatusForbidden)

	MakeRequest(t, NewRequest(t, "DELETE", workflowURL).AddTokenAuth(token), http.StatusNoContent)
	MakeRequest(t, NewRequest(t, "GET", workflowURL).AddTokenAuth(token), http.StatusNotFound)

	t.Run("CreateClosedIssueThatWorkflowCloses", func(t *testing.T) {
		req := NewRequestWithJSON(t, "POST", listURL, &api.CreateProjectWorkflowOption{Event: "item_added_to_project", Actions: api.ProjectWorkflowActions{IssueState: "close"}}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusCreated)
		req = NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo1/issues", &api.CreateIssueOption{Title: "closed", Projects: []int64{1}, Closed: true}).AddTokenAuth(token)
		assert.Equal(t, api.StateClosed, DecodeJSON(t, MakeRequest(t, req, http.StatusCreated), &api.Issue{}).State)
	})

	t.Run("Organization", func(t *testing.T) {
		orgProject := &project_model.Project{Title: "org workflows", OwnerID: 3, CreatorID: 2, Type: project_model.TypeOrganization}
		require.NoError(t, project_model.NewProject(t.Context(), orgProject))
		column := &project_model.Column{Title: "Done", ProjectID: orgProject.ID}
		require.NoError(t, project_model.NewColumn(t.Context(), column))
		orgListURL := fmt.Sprintf("/api/v1/orgs/org3/projects/%d/workflows", orgProject.ID)
		req := NewRequestWithJSON(t, "POST", orgListURL, &api.CreateProjectWorkflowOption{
			Event:   "item_closed",
			Actions: api.ProjectWorkflowActions{ColumnID: column.ID},
		}).AddTokenAuth(token)
		created := DecodeJSON(t, MakeRequest(t, req, http.StatusCreated), &api.ProjectWorkflow{})
		listed := DecodeJSON(t, MakeRequest(t, NewRequest(t, "GET", orgListURL).AddTokenAuth(token), http.StatusOK), []*api.ProjectWorkflow{})
		require.Len(t, listed, 1)
		assert.Equal(t, created.ID, listed[0].ID)
	})
}
