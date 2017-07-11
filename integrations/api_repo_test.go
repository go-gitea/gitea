// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"strings"
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

type searchResponseBody struct {
	ok   bool
	data []api.Repository
}

func TestAPISearchRepoNotLogin(t *testing.T) {
	prepareTestEnv(t)
	const keyword = "test"

	req := NewRequestf(t, "GET", "/api/v1/repos/search?q=%s", keyword)
	resp := MakeRequest(t, req, http.StatusOK)

	var body searchResponseBody
	DecodeJSON(t, resp, &body)
	for _, repo := range body.data {
		assert.True(t, strings.Contains(repo.Name, keyword))
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
