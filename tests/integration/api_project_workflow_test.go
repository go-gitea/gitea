// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	issues_model "gitea.dev/models/issues"
	project_model "gitea.dev/models/project"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIProjectWorkflows(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	project, columns := newRepo1WorkflowTestProject(t, "Done")
	backlog, done := columns[0], columns[1]
	label := &issues_model.Label{RepoID: 1, Name: "workflow", Color: "#0055ff"}
	require.NoError(t, issues_model.NewLabel(t.Context(), label))
	token := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteIssue, auth_model.AccessTokenScopeWriteOrganization)
	listURL := fmt.Sprintf("/api/v1/repos/user2/repo1/projects/%d/workflows", project.ID)

	req := NewRequestWithJSON(t, "POST", listURL, &api.CreateProjectWorkflowOption{
		Event:   "item_column_changed",
		Filters: api.ProjectWorkflowFilters{IssueType: "issue", TargetColumnID: done.ID},
		Actions: api.ProjectWorkflowActions{AddLabelIDs: []int64{label.ID}, IssueState: "close"},
	}).AddTokenAuth(token)
	created := DecodeJSON(t, MakeRequest(t, req, http.StatusCreated), &api.ProjectWorkflow{})
	assert.Equal(t, "item_column_changed", created.Event)
	assert.True(t, created.Enabled)
	assert.Equal(t, api.ProjectWorkflowFilters{IssueType: "issue", TargetColumnID: done.ID}, created.Filters)
	assert.Equal(t, api.ProjectWorkflowActions{AddLabelIDs: []int64{label.ID}, IssueState: "close"}, created.Actions)
	workflowURL := fmt.Sprintf("%s/%d", listURL, created.ID)

	fetched := DecodeJSON(t, MakeRequest(t, NewRequest(t, "GET", workflowURL).AddTokenAuth(token), http.StatusOK), &api.ProjectWorkflow{})
	assert.Equal(t, created.Filters, fetched.Filters)
	assert.Equal(t, created.Actions, fetched.Actions)

	listed := DecodeJSON(t, MakeRequest(t, NewRequest(t, "GET", listURL).AddTokenAuth(token), http.StatusOK), []*api.ProjectWorkflow{})
	require.Len(t, listed, 1)
	assert.Equal(t, created.ID, listed[0].ID)

	edit := func(opts *api.EditProjectWorkflowOption) *api.ProjectWorkflow {
		req := NewRequestWithJSON(t, "PATCH", workflowURL, opts).AddTokenAuth(token)
		return DecodeJSON(t, MakeRequest(t, req, http.StatusOK), &api.ProjectWorkflow{})
	}
	updated := edit(&api.EditProjectWorkflowOption{Filters: &api.ProjectWorkflowFilters{SourceColumnID: backlog.ID}})
	assert.Equal(t, api.ProjectWorkflowFilters{SourceColumnID: backlog.ID}, updated.Filters)
	assert.Equal(t, created.Actions, updated.Actions)

	updated = edit(&api.EditProjectWorkflowOption{Actions: &api.ProjectWorkflowActions{RemoveLabelIDs: []int64{label.ID}}})
	assert.Equal(t, api.ProjectWorkflowFilters{SourceColumnID: backlog.ID}, updated.Filters)
	assert.Equal(t, api.ProjectWorkflowActions{RemoveLabelIDs: []int64{label.ID}}, updated.Actions)

	updated = edit(&api.EditProjectWorkflowOption{Enabled: new(false)})
	assert.False(t, updated.Enabled)
	assert.Equal(t, api.ProjectWorkflowActions{RemoveLabelIDs: []int64{label.ID}}, updated.Actions)
	fetched = DecodeJSON(t, MakeRequest(t, NewRequest(t, "GET", workflowURL).AddTokenAuth(token), http.StatusOK), &api.ProjectWorkflow{})
	assert.False(t, fetched.Enabled)

	req = NewRequestWithJSON(t, "PATCH", workflowURL, &api.EditProjectWorkflowOption{Filters: &api.ProjectWorkflowFilters{LabelIDs: []int64{999999}}}).AddTokenAuth(token)
	assert.Contains(t, MakeRequest(t, req, http.StatusUnprocessableEntity).Body.String(), "invalid label")

	outsider := getUserToken(t, "user4", auth_model.AccessTokenScopeWriteIssue)
	req = NewRequestWithJSON(t, "POST", listURL, &api.CreateProjectWorkflowOption{Event: "item_opened", Actions: api.ProjectWorkflowActions{ColumnID: done.ID}}).AddTokenAuth(outsider)
	MakeRequest(t, req, http.StatusForbidden)
	MakeRequest(t, NewRequest(t, "DELETE", workflowURL).AddTokenAuth(outsider), http.StatusForbidden)

	MakeRequest(t, NewRequest(t, "DELETE", workflowURL).AddTokenAuth(token), http.StatusNoContent)
	MakeRequest(t, NewRequest(t, "GET", workflowURL).AddTokenAuth(token), http.StatusNotFound)

	t.Run("Organization", func(t *testing.T) {
		orgProject := &project_model.Project{Title: "org workflows", OwnerID: 3, CreatorID: 2, Type: project_model.TypeOrganization}
		orgColumns := newWorkflowTestProject(t, orgProject, "Done")
		orgListURL := fmt.Sprintf("/api/v1/orgs/org3/projects/%d/workflows", orgProject.ID)
		req := NewRequestWithJSON(t, "POST", orgListURL, &api.CreateProjectWorkflowOption{
			Event:   "item_closed",
			Actions: api.ProjectWorkflowActions{ColumnID: orgColumns[1].ID},
		}).AddTokenAuth(token)
		created := DecodeJSON(t, MakeRequest(t, req, http.StatusCreated), &api.ProjectWorkflow{})
		listed := DecodeJSON(t, MakeRequest(t, NewRequest(t, "GET", orgListURL).AddTokenAuth(token), http.StatusOK), []*api.ProjectWorkflow{})
		require.Len(t, listed, 1)
		assert.Equal(t, created.ID, listed[0].ID)
	})
}
