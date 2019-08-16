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
	prepareTestEnv(t)

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
	prepareTestEnv(t)
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

func TestAPICreateIssueID(t *testing.T) {
	prepareTestEnv(t)
	const body, firstTitle, dupTitle, freeTitle, notAllowedTitle = "apiTestBody", "apiTestTitle-first", "apiTestTitle-dup", "apiTestTitle-free", "apiTestTitle-notAllowed"
	const firstIndex = int64(3000)

	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	admin := models.AssertExistsAndLoadBean(t, &models.User{ID: 1}).(*models.User)
	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	assert.True(t, admin.IsAdmin)
	assert.False(t, owner.IsAdmin)

	session := loginUser(t, admin.Name)
	token := getTokenForLoggedInUser(t, session)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues?state=all&token=%s", owner.Name, repo.Name, token)

	// Must create with index 3000
	req := NewRequestWithJSON(t, "POST", urlStr, &api.CreateIssueOption{
		Index:    firstIndex,
		Body:     body,
		Title:    firstTitle,
		Assignee: owner.Name,
	})
	resp := session.MakeRequest(t, req, http.StatusCreated)
	var apiIssue api.Issue
	DecodeJSON(t, resp, &apiIssue)
	assert.Equal(t, apiIssue.Index, firstIndex)
	assert.Equal(t, apiIssue.Body, body)
	assert.Equal(t, apiIssue.Title, firstTitle)

	// Must fail
	req = NewRequestWithJSON(t, "POST", urlStr, &api.CreateIssueOption{
		Index:    firstIndex,
		Body:     body,
		Title:    dupTitle,
		Assignee: owner.Name,
	})
	resp = session.MakeRequest(t, req, http.StatusInternalServerError)

	// Must be the first one created
	models.AssertExistsAndLoadBean(t, &models.Issue{
		Index:      firstIndex,
		RepoID:     repo.ID,
		AssigneeID: owner.ID,
		Content:    body,
		Title:      firstTitle,
	})

	// Must create with index firstIndex + 1
	req = NewRequestWithJSON(t, "POST", urlStr, &api.CreateIssueOption{
		Body:     body,
		Title:    freeTitle,
		Assignee: owner.Name,
	})
	resp = session.MakeRequest(t, req, http.StatusCreated)
	DecodeJSON(t, resp, &apiIssue)
	assert.Equal(t, apiIssue.Index, firstIndex+1)
	assert.Equal(t, apiIssue.Body, body)
	assert.Equal(t, apiIssue.Title, freeTitle)

	// Must be the last one created
	models.AssertExistsAndLoadBean(t, &models.Issue{
		Index:      firstIndex + 1,
		RepoID:     repo.ID,
		AssigneeID: owner.ID,
		Content:    body,
		Title:      freeTitle,
	})

	session = loginUser(t, owner.Name)
	token = getTokenForLoggedInUser(t, session)
	urlStr = fmt.Sprintf("/api/v1/repos/%s/%s/issues?state=all&token=%s", owner.Name, repo.Name, token)

	// Must fail create with index
	req = NewRequestWithJSON(t, "POST", urlStr, &api.CreateIssueOption{
		Index:    firstIndex + 2,
		Body:     body,
		Title:    notAllowedTitle,
		Assignee: owner.Name,
	})
	resp = session.MakeRequest(t, req, http.StatusBadRequest)
}
