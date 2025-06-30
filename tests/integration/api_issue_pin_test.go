// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIPinIssue(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	assert.NoError(t, unittest.LoadFixtures())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: repo.ID})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)

	// Pin the Issue
	req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/pin", repo.OwnerName, repo.Name, issue.Index)).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)

	// Check if the Issue is pinned
	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d", repo.OwnerName, repo.Name, issue.Index))
	resp := MakeRequest(t, req, http.StatusOK)
	var issueAPI api.Issue
	DecodeJSON(t, resp, &issueAPI)
	assert.Equal(t, 1, issueAPI.PinOrder)
}

func TestAPIUnpinIssue(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	assert.NoError(t, unittest.LoadFixtures())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: repo.ID})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)

	// Pin the Issue
	req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/pin", repo.OwnerName, repo.Name, issue.Index)).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)

	// Check if the Issue is pinned
	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d", repo.OwnerName, repo.Name, issue.Index))
	resp := MakeRequest(t, req, http.StatusOK)
	var issueAPI api.Issue
	DecodeJSON(t, resp, &issueAPI)
	assert.Equal(t, 1, issueAPI.PinOrder)

	// Unpin the Issue
	req = NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/pin", repo.OwnerName, repo.Name, issue.Index)).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)

	// Check if the Issue is no longer pinned
	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d", repo.OwnerName, repo.Name, issue.Index))
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &issueAPI)
	assert.Equal(t, 0, issueAPI.PinOrder)
}

func TestAPIMoveIssuePin(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	assert.NoError(t, unittest.LoadFixtures())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: repo.ID})
	issue2 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2, RepoID: repo.ID})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)

	// Pin the first Issue
	req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/pin", repo.OwnerName, repo.Name, issue.Index)).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)

	// Check if the first Issue is pinned at position 1
	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d", repo.OwnerName, repo.Name, issue.Index))
	resp := MakeRequest(t, req, http.StatusOK)
	var issueAPI api.Issue
	DecodeJSON(t, resp, &issueAPI)
	assert.Equal(t, 1, issueAPI.PinOrder)

	// Pin the second Issue
	req = NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/pin", repo.OwnerName, repo.Name, issue2.Index)).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)

	// Move the first Issue to position 2
	req = NewRequest(t, "PATCH", fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/pin/2", repo.OwnerName, repo.Name, issue.Index)).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)

	// Check if the first Issue is pinned at position 2
	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d", repo.OwnerName, repo.Name, issue.Index))
	resp = MakeRequest(t, req, http.StatusOK)
	var issueAPI3 api.Issue
	DecodeJSON(t, resp, &issueAPI3)
	assert.Equal(t, 2, issueAPI3.PinOrder)

	// Check if the second Issue is pinned at position 1
	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d", repo.OwnerName, repo.Name, issue2.Index))
	resp = MakeRequest(t, req, http.StatusOK)
	var issueAPI4 api.Issue
	DecodeJSON(t, resp, &issueAPI4)
	assert.Equal(t, 1, issueAPI4.PinOrder)
}

func TestAPIListPinnedIssues(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	assert.NoError(t, unittest.LoadFixtures())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: repo.ID})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteIssue)

	// Pin the Issue
	req := NewRequest(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/pin", repo.OwnerName, repo.Name, issue.Index)).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)

	// Check if the Issue is in the List
	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/issues/pinned", repo.OwnerName, repo.Name))
	resp := MakeRequest(t, req, http.StatusOK)
	var issueList []api.Issue
	DecodeJSON(t, resp, &issueList)

	assert.Len(t, issueList, 1)
	assert.Equal(t, issue.ID, issueList[0].ID)
}

func TestAPIListPinnedPullrequests(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	assert.NoError(t, unittest.LoadFixtures())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/pulls/pinned", repo.OwnerName, repo.Name))
	resp := MakeRequest(t, req, http.StatusOK)
	var prList []api.PullRequest
	DecodeJSON(t, resp, &prList)

	assert.Empty(t, prList)
}

func TestAPINewPinAllowed(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/new_pin_allowed", owner.Name, repo.Name))
	resp := MakeRequest(t, req, http.StatusOK)

	var newPinsAllowed api.NewIssuePinsAllowed
	DecodeJSON(t, resp, &newPinsAllowed)

	assert.True(t, newPinsAllowed.Issues)
	assert.True(t, newPinsAllowed.PullRequests)
}
