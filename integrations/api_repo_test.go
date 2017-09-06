// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/sdk/gitea"

	"github.com/stretchr/testify/assert"
)

func TestAPIUserReposNotLogin(t *testing.T) {
	prepareTestEnv(t)
	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)

	req := NewRequestf(t, "GET", "/api/v1/users/%s/repos", user.Name)
	resp := MakeRequest(t, req, http.StatusOK)

	var apiRepos []api.Repository
	DecodeJSON(t, resp, &apiRepos)
	expectedLen := models.GetCount(t, models.Repository{OwnerID: user.ID},
		models.Cond("is_private = ?", false))
	assert.Len(t, apiRepos, expectedLen)
	for _, repo := range apiRepos {
		assert.EqualValues(t, user.ID, repo.Owner.ID)
		assert.False(t, repo.Private)
	}
}

func TestAPISearchRepoNotLogin(t *testing.T) {
	prepareTestEnv(t)
	const keyword = "test"

	req := NewRequestf(t, "GET", "/api/v1/repos/search?q=%s", keyword)
	resp := MakeRequest(t, req, http.StatusOK)

	var body api.SearchResults
	DecodeJSON(t, resp, &body)
	assert.NotEmpty(t, body.Data)
	for _, repo := range body.Data {
		assert.Contains(t, repo.Name, keyword)
		assert.False(t, repo.Private)
	}

	// Should return all (max 50) public repositories
	req = NewRequest(t, "GET", "/api/v1/repos/search?limit=50")
	resp = MakeRequest(t, req, http.StatusOK)

	DecodeJSON(t, resp, &body)
	assert.Len(t, body.Data, 12)
	for _, repo := range body.Data {
		assert.NotEmpty(t, repo.Name)
		assert.False(t, repo.Private)
	}

	// Should return (max 10) public repositories
	req = NewRequest(t, "GET", "/api/v1/repos/search?limit=10")
	resp = MakeRequest(t, req, http.StatusOK)

	DecodeJSON(t, resp, &body)
	assert.Len(t, body.Data, 10)
	for _, repo := range body.Data {
		assert.NotEmpty(t, repo.Name)
		assert.False(t, repo.Private)
	}

	const keyword2 = "big_test_"
	// Should return all public repositories which (partial) match keyword
	req = NewRequestf(t, "GET", "/api/v1/repos/search?q=%s", keyword2)
	resp = MakeRequest(t, req, http.StatusOK)

	DecodeJSON(t, resp, &body)
	assert.Len(t, body.Data, 4)
	for _, repo := range body.Data {
		assert.Contains(t, repo.Name, keyword2)
		assert.False(t, repo.Private)
	}

	// Should return all public repositories accessible and related to user
	const userID = int64(15)
	req = NewRequestf(t, "GET", "/api/v1/repos/search?uid=%d", userID)
	resp = MakeRequest(t, req, http.StatusOK)

	DecodeJSON(t, resp, &body)
	assert.Len(t, body.Data, 4)
	for _, repo := range body.Data {
		assert.NotEmpty(t, repo.Name)
		assert.False(t, repo.Private)
	}

	// Should return all public repositories accessible and related to user
	const user2ID = int64(16)
	req = NewRequestf(t, "GET", "/api/v1/repos/search?uid=%d", user2ID)
	resp = MakeRequest(t, req, http.StatusOK)

	DecodeJSON(t, resp, &body)
	assert.Len(t, body.Data, 1)
	for _, repo := range body.Data {
		assert.NotEmpty(t, repo.Name)
		assert.False(t, repo.Private)
	}

	// Should return all public repositories owned by organization
	const orgID = int64(17)
	req = NewRequestf(t, "GET", "/api/v1/repos/search?uid=%d", orgID)
	resp = MakeRequest(t, req, http.StatusOK)

	DecodeJSON(t, resp, &body)
	assert.Len(t, body.Data, 1)
	for _, repo := range body.Data {
		assert.NotEmpty(t, repo.Name)
		assert.Equal(t, repo.Owner.ID, orgID)
		assert.False(t, repo.Private)
	}
}

func TestAPISearchRepoLoggedUser(t *testing.T) {
	prepareTestEnv(t)

	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 15}).(*models.User)
	user2 := models.AssertExistsAndLoadBean(t, &models.User{ID: 16}).(*models.User)
	session := loginUser(t, user.Name)
	session2 := loginUser(t, user2.Name)

	var body api.SearchResults

	// Get public repositories accessible and not related to logged in user that match the keyword
	// Should return all public repositories which (partial) match keyword
	const keyword = "big_test_"
	req := NewRequestf(t, "GET", "/api/v1/repos/search?q=%s", keyword)
	resp := session.MakeRequest(t, req, http.StatusOK)

	DecodeJSON(t, resp, &body)
	assert.Len(t, body.Data, 4)
	for _, repo := range body.Data {
		assert.Contains(t, repo.Name, keyword)
		assert.False(t, repo.Private)
	}
	// Test when user2 is logged in
	resp = session2.MakeRequest(t, req, http.StatusOK)

	DecodeJSON(t, resp, &body)
	assert.Len(t, body.Data, 4)
	for _, repo := range body.Data {
		assert.Contains(t, repo.Name, keyword)
		assert.False(t, repo.Private)
	}

	// Get all public repositories accessible and not related to logged in user
	// Should return all (max 50) public repositories
	req = NewRequest(t, "GET", "/api/v1/repos/search?limit=50")
	resp = session.MakeRequest(t, req, http.StatusOK)

	DecodeJSON(t, resp, &body)
	assert.Len(t, body.Data, 12)
	for _, repo := range body.Data {
		assert.NotEmpty(t, repo.Name)
		assert.False(t, repo.Private)
	}
	// Test when user2 is logged in
	resp = session2.MakeRequest(t, req, http.StatusOK)

	DecodeJSON(t, resp, &body)
	assert.Len(t, body.Data, 12)
	for _, repo := range body.Data {
		assert.NotEmpty(t, repo.Name)
		assert.False(t, repo.Private)
	}

	// Get all public repositories accessible and not related to logged in user
	// Should return all (max 10) public repositories
	req = NewRequest(t, "GET", "/api/v1/repos/search?limit=10")
	resp = session.MakeRequest(t, req, http.StatusOK)

	DecodeJSON(t, resp, &body)
	assert.Len(t, body.Data, 10)
	for _, repo := range body.Data {
		assert.NotEmpty(t, repo.Name)
		assert.False(t, repo.Private)
	}
	// Test when user2 is logged in
	resp = session2.MakeRequest(t, req, http.StatusOK)

	DecodeJSON(t, resp, &body)
	assert.Len(t, body.Data, 10)
	for _, repo := range body.Data {
		assert.NotEmpty(t, repo.Name)
		assert.False(t, repo.Private)
	}

	// Get repositories of logged in user
	// Should return all public and private repositories accessible and related to user
	req = NewRequestf(t, "GET", "/api/v1/repos/search?uid=%d", user.ID)
	resp = session.MakeRequest(t, req, http.StatusOK)

	DecodeJSON(t, resp, &body)
	assert.Len(t, body.Data, 8)
	for _, repo := range body.Data {
		assert.NotEmpty(t, repo.Name)
	}
	// Test when user2 is logged in
	req = NewRequestf(t, "GET", "/api/v1/repos/search?uid=%d", user2.ID)
	resp = session2.MakeRequest(t, req, http.StatusOK)

	DecodeJSON(t, resp, &body)
	assert.Len(t, body.Data, 2)
	for _, repo := range body.Data {
		assert.NotEmpty(t, repo.Name)
	}

	// Get repositories of another user
	// Should return all public repositories accessible and related to user
	req = NewRequestf(t, "GET", "/api/v1/repos/search?uid=%d", user2.ID)
	resp = session.MakeRequest(t, req, http.StatusOK)

	DecodeJSON(t, resp, &body)
	assert.Len(t, body.Data, 1)
	for _, repo := range body.Data {
		assert.NotEmpty(t, repo.Name)
		assert.False(t, repo.Private)
	}
	// Test when user2 is logged in
	req = NewRequestf(t, "GET", "/api/v1/repos/search?uid=%d", user.ID)
	resp = session2.MakeRequest(t, req, http.StatusOK)

	DecodeJSON(t, resp, &body)
	assert.Len(t, body.Data, 4)
	for _, repo := range body.Data {
		assert.NotEmpty(t, repo.Name)
		assert.False(t, repo.Private)
	}

	// Get repositories of organization owned by logged in user
	// Should return all public and private repositories owned by organization
	const orgID = int64(17)
	req = NewRequestf(t, "GET", "/api/v1/repos/search?uid=%d", orgID)
	resp = session.MakeRequest(t, req, http.StatusOK)

	DecodeJSON(t, resp, &body)
	assert.Len(t, body.Data, 2)
	for _, repo := range body.Data {
		assert.NotEmpty(t, repo.Name)
		assert.Equal(t, repo.Owner.ID, orgID)
	}

	// Get repositories of organization owned by another user
	// Should return all public repositories owned by organization
	req = NewRequestf(t, "GET", "/api/v1/repos/search?uid=%d", orgID)
	resp = session2.MakeRequest(t, req, http.StatusOK)

	DecodeJSON(t, resp, &body)
	assert.Len(t, body.Data, 1)
	for _, repo := range body.Data {
		assert.NotEmpty(t, repo.Name)
		assert.Equal(t, repo.Owner.ID, orgID)
		assert.False(t, repo.Private)
	}
}

func TestAPIViewRepo(t *testing.T) {
	prepareTestEnv(t)

	req := NewRequest(t, "GET", "/api/v1/repos/user2/repo1")
	resp := MakeRequest(t, req, http.StatusOK)

	var repo api.Repository
	DecodeJSON(t, resp, &repo)
	assert.EqualValues(t, 1, repo.ID)
	assert.EqualValues(t, "repo1", repo.Name)
}

func TestAPIOrgRepos(t *testing.T) {
	prepareTestEnv(t)
	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	// User3 is an Org. Check their repos.
	sourceOrg := models.AssertExistsAndLoadBean(t, &models.User{ID: 3}).(*models.User)
	// Login as User2.
	session := loginUser(t, user.Name)

	req := NewRequestf(t, "GET", "/api/v1/orgs/%s/repos", sourceOrg.Name)
	resp := session.MakeRequest(t, req, http.StatusOK)

	var apiRepos []*api.Repository
	DecodeJSON(t, resp, &apiRepos)
	expectedLen := models.GetCount(t, models.Repository{OwnerID: sourceOrg.ID},
		models.Cond("is_private = ?", false))
	assert.Len(t, apiRepos, expectedLen)
	for _, repo := range apiRepos {
		assert.False(t, repo.Private)
	}
}

func TestAPIGetRepoByIDUnauthorized(t *testing.T) {
	prepareTestEnv(t)
	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 4}).(*models.User)
	sess := loginUser(t, user.Name)
	req := NewRequestf(t, "GET", "/api/v1/repositories/2")
	sess.MakeRequest(t, req, http.StatusNotFound)
}
