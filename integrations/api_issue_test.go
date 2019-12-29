// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestAPIListIssues(t *testing.T) {
	defer prepareTestEnv(t)()

	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session)
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/issues?state=all&token=%s",
		owner.Name, repo.Name, token)
	resp := session.MakeRequest(t, req, http.StatusOK)
	var apiIssues []*api.Issue
	DecodeJSON(t, resp, &apiIssues)
	assert.Len(t, apiIssues, models.GetCount(t, &models.Issue{RepoID: repo.ID}))
	for _, apiIssue := range apiIssues {
		models.AssertExistsAndLoadBean(t, &models.Issue{ID: apiIssue.ID, RepoID: repo.ID})
	}
}

func TestAPICreateIssue(t *testing.T) {
	defer prepareTestEnv(t)()
	const body, title = "apiTestBody", "apiTestTitle"

	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues?state=all&token=%s", owner.Name, repo.Name, token)
	req := NewRequestWithJSON(t, "POST", urlStr, &api.CreateIssueOption{
		Body:     body,
		Title:    title,
		Assignee: owner.Name,
	})
	resp := session.MakeRequest(t, req, http.StatusCreated)
	var apiIssue api.Issue
	DecodeJSON(t, resp, &apiIssue)
	assert.Equal(t, apiIssue.Body, body)
	assert.Equal(t, apiIssue.Title, title)

	models.AssertExistsAndLoadBean(t, &models.Issue{
		RepoID:     repo.ID,
		AssigneeID: owner.ID,
		Content:    body,
		Title:      title,
	})
}

func TestAPIEditIssue(t *testing.T) {
	defer prepareTestEnv(t)()

	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 2}).(*models.Repository)
	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)
	issueBefore := models.AssertExistsAndLoadBean(t, &models.Issue{ID: 10}).(*models.Issue)

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session)

	issueState := "closed"

	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d?token=%s", owner.Name, repo.Name, issueBefore.Index, token)
	req := NewRequestWithJSON(t, "PATCH", urlStr, api.EditIssueOption{
		State: &issueState,
		// ToDo change more
	})
	resp := session.MakeRequest(t, req, http.StatusCreated)
	var apiIssue api.Issue
	DecodeJSON(t, resp, &apiIssue)

	issueAfter := models.AssertExistsAndLoadBean(t, &models.Issue{ID: 10}).(*models.Issue)

	assert.Equal(t, api.StateOpen, issueBefore.State())
	assert.Equal(t, api.StateClosed, issueAfter.State())
	// check deleted user
	assert.Equal(t, 100, issueBefore.Poster.ID)
	assert.Equal(t, 100, apiIssue.Poster.ID)
	assert.Equal(t, 100, issueAfter.Poster.ID)
}
