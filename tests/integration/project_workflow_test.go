// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"slices"
	"testing"

	auth_model "gitea.dev/models/auth"
	issues_model "gitea.dev/models/issues"
	project_model "gitea.dev/models/project"
	"gitea.dev/models/unittest"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newWorkflowTestProject returns a "Backlog" default column followed by the named ones, so a
// card a workflow failed to move is never mistaken for one it moved
func newWorkflowTestProject(t *testing.T, project *project_model.Project, titles ...string) []*project_model.Column {
	t.Helper()
	require.NoError(t, project_model.NewProject(t.Context(), project))
	columns := []*project_model.Column{{Title: "Backlog", ProjectID: project.ID, Default: true}}
	for _, title := range titles {
		columns = append(columns, &project_model.Column{Title: title, ProjectID: project.ID})
	}
	for _, column := range columns {
		require.NoError(t, project_model.NewColumn(t.Context(), column))
	}
	return columns
}

func newRepo1WorkflowTestProject(t *testing.T, titles ...string) (*project_model.Project, []*project_model.Column) {
	t.Helper()
	project := &project_model.Project{Title: "workflows", RepoID: 1, CreatorID: 2, Type: project_model.TypeRepository}
	return project, newWorkflowTestProject(t, project, titles...)
}

func TestProjectWorkflowsWeb(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	project, columns := newRepo1WorkflowTestProject(t, "Done")
	label := &issues_model.Label{RepoID: 1, Name: "workflow", Color: "#0055ff"}
	require.NoError(t, issues_model.NewLabel(t.Context(), label))
	link := fmt.Sprintf("/user2/repo1/projects/%d/workflows", project.ID)
	session := loginUser(t, "user2")

	canWrite := func(session *TestSession) string {
		resp := session.MakeRequest(t, NewRequest(t, "GET", link), http.StatusOK)
		return NewHTMLParser(t, resp.Body).Find("#project-workflows").AttrOr("data-can-write", "")
	}
	assert.Equal(t, "true", canWrite(session))
	assert.Equal(t, "false", canWrite(nil))
	session.MakeRequest(t, NewRequest(t, "GET", link+"/item_closed"), http.StatusOK)
	session.MakeRequest(t, NewRequest(t, "GET", link+"/no_such_event"), http.StatusNotFound)
	session.MakeRequest(t, NewRequest(t, "GET", link+"/999999"), http.StatusNotFound)

	req := NewRequestWithJSON(t, "POST", link, &api.CreateProjectWorkflowOption{
		Event:   "item_closed",
		Actions: api.ProjectWorkflowActions{ColumnID: columns[1].ID},
	})
	workflow := DecodeJSON(t, session.MakeRequest(t, req, http.StatusOK), &api.ProjectWorkflow{})
	assert.Equal(t, api.ProjectWorkflowActions{ColumnID: columns[1].ID}, workflow.Actions)
	workflowLink := fmt.Sprintf("%s/%d", link, workflow.ID)
	session.MakeRequest(t, NewRequest(t, "GET", workflowLink), http.StatusOK)

	type workflowsData struct {
		Events []struct {
			Event string `json:"event"`
		} `json:"events"`
		Workflows []*api.ProjectWorkflow `json:"workflows"`
		Columns   []struct {
			ID    int64  `json:"id"`
			Title string `json:"title"`
		} `json:"columns"`
		Labels []*api.Label `json:"labels"`
	}
	data := DecodeJSON(t, MakeRequest(t, NewRequest(t, "GET", link+"/data"), http.StatusOK), &workflowsData{})
	assert.Len(t, data.Events, 9)
	require.Len(t, data.Workflows, 1)
	assert.Equal(t, workflow.ID, data.Workflows[0].ID)
	require.Len(t, data.Columns, 2)
	assert.Equal(t, columns[1].ID, data.Columns[1].ID)
	assert.Equal(t, "Done", data.Columns[1].Title)
	assert.True(t, slices.ContainsFunc(data.Labels, func(l *api.Label) bool { return l.ID == label.ID }))

	req = NewRequestWithJSON(t, "POST", workflowLink, &api.EditProjectWorkflowOption{Enabled: new(false)})
	updated := DecodeJSON(t, session.MakeRequest(t, req, http.StatusOK), &api.ProjectWorkflow{})
	assert.False(t, updated.Enabled)
	assert.Equal(t, workflow.Actions, updated.Actions)

	req = NewRequestWithJSON(t, "POST", link, &api.CreateProjectWorkflowOption{Event: "item_closed"})
	resp := session.MakeRequest(t, req, http.StatusBadRequest)
	assert.Equal(t, "At least one action must be configured", DecodeJSON(t, resp, map[string]any{})["errorMessage"])

	loginUser(t, "user4").MakeRequest(t, NewRequest(t, "POST", workflowLink+"/delete"), http.StatusNotFound)

	session.MakeRequest(t, NewRequest(t, "POST", workflowLink+"/delete"), http.StatusOK)
	session.MakeRequest(t, NewRequest(t, "GET", workflowLink), http.StatusNotFound)
}

func TestProjectWorkflowExecution(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	project, columns := newRepo1WorkflowTestProject(t, "In Progress", "Done")
	inProgress, done := columns[1], columns[2]
	label := &issues_model.Label{RepoID: 1, Name: "triaged", Color: "#00ff00"}
	require.NoError(t, issues_model.NewLabel(t.Context(), label))
	link := fmt.Sprintf("/user2/repo1/projects/%d", project.ID)
	session := loginUser(t, "user2")

	for _, opts := range []*api.CreateProjectWorkflowOption{{
		Event:   "item_opened",
		Actions: api.ProjectWorkflowActions{ColumnID: inProgress.ID, AddLabelIDs: []int64{label.ID}},
	}, {
		Event:   "item_column_changed",
		Filters: api.ProjectWorkflowFilters{TargetColumnID: done.ID},
		Actions: api.ProjectWorkflowActions{IssueState: "close"},
	}} {
		session.MakeRequest(t, NewRequestWithJSON(t, "POST", link+"/workflows", opts), http.StatusOK)
	}

	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)
	req := NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo1/issues", &api.CreateIssueOption{
		Title:    "workflow item",
		Projects: []int64{project.ID},
	}).AddTokenAuth(token)
	issueID := DecodeJSON(t, MakeRequest(t, req, http.StatusCreated), &api.Issue{}).ID

	card := unittest.AssertExistsAndLoadBean(t, &project_model.ProjectIssue{IssueID: issueID, ProjectID: project.ID})
	assert.Equal(t, inProgress.ID, card.ProjectColumnID)
	unittest.AssertExistsAndLoadBean(t, &issues_model.IssueLabel{IssueID: issueID, LabelID: label.ID})
	assert.False(t, unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: issueID}).IsClosed)

	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("%s/%d/move", link, done.ID), map[string]any{
		"issues": []map[string]any{{"issueID": issueID, "sorting": 0}},
	})
	session.MakeRequest(t, req, http.StatusOK)

	card = unittest.AssertExistsAndLoadBean(t, &project_model.ProjectIssue{IssueID: issueID, ProjectID: project.ID})
	assert.Equal(t, done.ID, card.ProjectColumnID)
	assert.True(t, unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: issueID}).IsClosed)
}
