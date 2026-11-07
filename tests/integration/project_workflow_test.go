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
	"gitea.dev/models/unittest"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestProjectWorkflowsWeb(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	link := "/user2/repo1/projects/1/workflows"
	session := loginUser(t, "user2")

	canWrite := func(session *TestSession) string {
		resp := session.MakeRequest(t, NewRequest(t, "GET", link), http.StatusOK)
		return NewHTMLParser(t, resp.Body).Find("#project-workflows").AttrOr("data-can-write", "")
	}
	assert.Equal(t, "true", canWrite(session))
	assert.Equal(t, "false", canWrite(nil))
	session.MakeRequest(t, NewRequest(t, "GET", link+"?event=item_closed"), http.StatusOK)
	session.MakeRequest(t, NewRequest(t, "GET", link+"?event=no_such_event"), http.StatusNotFound)
	session.MakeRequest(t, NewRequest(t, "GET", link+"/item_closed"), http.StatusNotFound)
	session.MakeRequest(t, NewRequest(t, "GET", link+"/999999"), http.StatusNotFound)

	req := NewRequestWithJSON(t, "POST", link, &api.CreateProjectWorkflowOption{
		Event:   "item_closed",
		Actions: api.ProjectWorkflowActions{ColumnID: 3},
	})
	workflow := DecodeJSON(t, session.MakeRequest(t, req, http.StatusOK), &api.ProjectWorkflow{})
	assert.Equal(t, api.ProjectWorkflowActions{ColumnID: 3}, workflow.Actions)
	workflowLink := fmt.Sprintf("%s/%d", link, workflow.ID)
	session.MakeRequest(t, NewRequest(t, "GET", workflowLink), http.StatusOK)

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
	t.Run("Execution", func(t *testing.T) {
		link := "/user2/repo1/projects/1"

		for _, opts := range []*api.CreateProjectWorkflowOption{{
			Event:   "item_added_to_project",
			Actions: api.ProjectWorkflowActions{ColumnID: 2, AddLabelIDs: []int64{1}},
		}, {
			Event:   "item_column_changed",
			Actions: api.ProjectWorkflowActions{IssueState: "close"},
		}} {
			session.MakeRequest(t, NewRequestWithJSON(t, "POST", link+"/workflows", opts), http.StatusOK)
		}

		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)
		req := NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo1/issues", &api.CreateIssueOption{
			Title:    "workflow item",
			Projects: []int64{1},
		}).AddTokenAuth(token)
		created := DecodeJSON(t, MakeRequest(t, req, http.StatusCreated), &api.Issue{})

		assert.EqualValues(t, 2, unittest.AssertExistsAndLoadBean(t, &project_model.ProjectIssue{IssueID: created.ID, ProjectID: 1}).ProjectColumnID)
		unittest.AssertExistsAndLoadBean(t, &issues_model.IssueLabel{IssueID: created.ID, LabelID: 1})
		assert.False(t, unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: created.ID}).IsClosed)
		resp := session.MakeRequest(t, NewRequestf(t, "GET", "/user2/repo1/issues/%d", created.Index), http.StatusOK)
		assert.Contains(t, NewHTMLParser(t, resp.Body).Find(".timeline-item.event .comment-text-line").Text(), "Project workflow Item added to project")

		req = NewRequestWithJSON(t, "POST", link+"/3/move", map[string]any{
			"issues": []map[string]any{{"issueID": created.ID, "sorting": 0}},
		})
		session.MakeRequest(t, req, http.StatusOK)

		assert.EqualValues(t, 3, unittest.AssertExistsAndLoadBean(t, &project_model.ProjectIssue{IssueID: created.ID, ProjectID: 1}).ProjectColumnID)
		assert.True(t, unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: created.ID}).IsClosed)
	})
}
