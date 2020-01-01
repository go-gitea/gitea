// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"testing"
	"time"

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

func TestAPIEditIssue(t *testing.T) {
	prepareTestEnv(t)

	issueBefore := models.AssertExistsAndLoadBean(t, &models.Issue{ID: 9}).(*models.Issue)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: issueBefore.RepoID}).(*models.Repository)
	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)
	assert.NoError(t, issueBefore.LoadAttributes())
	assert.Equal(t, int64(1019307200), int64(issueBefore.DeadlineUnix))
	assert.Equal(t, api.StateOpen, issueBefore.State())

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session)

	// update values of issue
	issueState := "closed"
	removeDeadline := time.Unix(0, 0)
	milestone := int64(4)
	body := "new content!"
	title := "new title from api set"

	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d?token=%s", owner.Name, repo.Name, issueBefore.Index, token)
	req := NewRequestWithJSON(t, "PATCH", urlStr, api.EditIssueOption{
		State:     &issueState,
		Deadline:  &removeDeadline,
		Milestone: &milestone,
		Body:      &body,
		Title:     title,

		// ToDo change more
	})
	resp := session.MakeRequest(t, req, http.StatusCreated)
	var apiIssue api.Issue
	DecodeJSON(t, resp, &apiIssue)

	issueAfter := models.AssertExistsAndLoadBean(t, &models.Issue{ID: 9}).(*models.Issue)

	// check deleted user
	assert.Equal(t, int64(500), issueAfter.PosterID)
	assert.NoError(t, issueAfter.LoadAttributes())
	assert.Equal(t, int64(-1), issueAfter.PosterID)
	assert.Equal(t, int64(-1), issueBefore.PosterID)
	assert.Equal(t, int64(-1), apiIssue.Poster.ID)

	// API response
	assert.Equal(t, api.StateClosed, apiIssue.State)
	assert.Equal(t, milestone, apiIssue.Milestone.ID)
	assert.Equal(t, body, apiIssue.Body)
	assert.True(t, apiIssue.Deadline == nil)
	assert.Equal(t, title, apiIssue.Title)

	// in database
	assert.Equal(t, api.StateClosed, issueAfter.State())
	assert.Equal(t, milestone, issueAfter.MilestoneID)
	assert.Equal(t, int64(0), int64(issueAfter.DeadlineUnix))
	assert.Equal(t, body, issueAfter.Content)
	assert.Equal(t, title, issueAfter.Title)
}
