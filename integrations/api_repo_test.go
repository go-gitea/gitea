// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
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

func TestAPISearchRepo(t *testing.T) {
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

	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 15}).(*models.User)
	user2 := models.AssertExistsAndLoadBean(t, &models.User{ID: 16}).(*models.User)
	user3 := models.AssertExistsAndLoadBean(t, &models.User{ID: 18}).(*models.User)
	orgUser := models.AssertExistsAndLoadBean(t, &models.User{ID: 17}).(*models.User)

	// Map of expected results, where key is user for login
	type expectedResults map[*models.User]struct {
		count           int
		repoOwnerID     int64
		repoName        string
		includesPrivate bool
	}

	testCases := []struct {
		name, requestURL string
		expectedResults
	}{
		{name: "RepositoriesMax50", requestURL: "/api/v1/repos/search?limit=50", expectedResults: expectedResults{
			nil:   {count: 12},
			user:  {count: 12},
			user2: {count: 12}},
		},
		{name: "RepositoriesMax10", requestURL: "/api/v1/repos/search?limit=10", expectedResults: expectedResults{
			nil:   {count: 10},
			user:  {count: 10},
			user2: {count: 10}},
		},
		{name: "RepositoriesDefaultMax10", requestURL: "/api/v1/repos/search", expectedResults: expectedResults{
			nil:   {count: 10},
			user:  {count: 10},
			user2: {count: 10}},
		},
		{name: "RepositoriesByName", requestURL: fmt.Sprintf("/api/v1/repos/search?q=%s", "big_test_"), expectedResults: expectedResults{
			nil:   {count: 4, repoName: "big_test_"},
			user:  {count: 4, repoName: "big_test_"},
			user2: {count: 4, repoName: "big_test_"}},
		},
		{name: "RepositoriesAccessibleAndRelatedToUser", requestURL: fmt.Sprintf("/api/v1/repos/search?uid=%d", user.ID), expectedResults: expectedResults{
			nil:   {count: 4},
			user:  {count: 8, includesPrivate: true},
			user2: {count: 4}},
		},
		{name: "RepositoriesAccessibleAndRelatedToUser2", requestURL: fmt.Sprintf("/api/v1/repos/search?uid=%d", user2.ID), expectedResults: expectedResults{
			nil:   {count: 1},
			user:  {count: 1},
			user2: {count: 2, includesPrivate: true}},
		},
		{name: "RepositoriesAccessibleAndRelatedToUser3", requestURL: fmt.Sprintf("/api/v1/repos/search?uid=%d", user3.ID), expectedResults: expectedResults{
			nil:   {count: 1},
			user:  {count: 1},
			user2: {count: 1},
			user3: {count: 4, includesPrivate: true}},
		},
		{name: "RepositoriesOwnedByOrganization", requestURL: fmt.Sprintf("/api/v1/repos/search?uid=%d", orgUser.ID), expectedResults: expectedResults{
			nil:   {count: 1, repoOwnerID: orgUser.ID},
			user:  {count: 2, repoOwnerID: orgUser.ID, includesPrivate: true},
			user2: {count: 1, repoOwnerID: orgUser.ID}},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			for userToLogin, expected := range testCase.expectedResults {
				var session *TestSession
				var testName string
				if userToLogin != nil && userToLogin.ID > 0 {
					testName = fmt.Sprintf("LoggedUser%d", userToLogin.ID)
					session = loginUser(t, userToLogin.Name)
				} else {
					testName = "AnonymousUser"
					session = emptyTestSession(t)
				}

				t.Run(testName, func(t *testing.T) {
					request := NewRequest(t, "GET", testCase.requestURL)
					response := session.MakeRequest(t, request, http.StatusOK)

					var body api.SearchResults
					DecodeJSON(t, response, &body)

					assert.Len(t, body.Data, expected.count)
					for _, repo := range body.Data {
						assert.NotEmpty(t, repo.Name)

						if len(expected.repoName) > 0 {
							assert.Contains(t, repo.Name, expected.repoName)
						}

						if expected.repoOwnerID > 0 {
							assert.Equal(t, expected.repoOwnerID, repo.Owner.ID)
						}

						if !expected.includesPrivate {
							assert.False(t, repo.Private)
						}
					}
				})
			}
		})
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
