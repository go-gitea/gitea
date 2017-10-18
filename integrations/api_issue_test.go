// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/sdk/gitea"

	"fmt"
	"github.com/stretchr/testify/assert"
)

func TestAPIListIssues(t *testing.T) {
	prepareTestEnv(t)

	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	session := loginUser(t, owner.Name)
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/issues?state=all",
		owner.Name, repo.Name)
	resp := session.MakeRequest(t, req, http.StatusOK)
	var apiIssues []*api.Issue
	DecodeJSON(t, resp, &apiIssues)
	assert.Len(t, apiIssues, models.GetCount(t, &models.Issue{RepoID: repo.ID}))
	for _, apiIssue := range apiIssues {
		models.AssertExistsAndLoadBean(t, &models.Issue{ID: apiIssue.ID, RepoID: repo.ID})
	}
}

func TestAPICreateIssue(t *testing.T) {
	prepareTestEnv(t)
	const body, title = "apiTestBody", "apiTestTitle"

	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	session := loginUser(t, owner.Name)

	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues?state=all", owner.Name, repo.Name)
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
