// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIWatch(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := "user1"
	repo := "user2/repo1"

	session := loginUser(t, user)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadUser)
	tokenWithRepoScope := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeReadUser)

	t.Run("Watch", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "PUT", fmt.Sprintf("/api/v1/repos/%s/subscription", repo)).
			AddTokenAuth(tokenWithRepoScope)
		MakeRequest(t, req, http.StatusOK)

		// blocked user can't watch a repo
		user34 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 34})
		req = NewRequest(t, "PUT", fmt.Sprintf("/api/v1/repos/%s/subscription", repo)).
			AddTokenAuth(getUserToken(t, user34.Name, auth_model.AccessTokenScopeWriteRepository))
		MakeRequest(t, req, http.StatusForbidden)
	})

	t.Run("GetWatchedRepos", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/users/%s/subscriptions", user)).
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, "1", resp.Header().Get("X-Total-Count"))

		repos := DecodeJSON(t, resp, []api.Repository{})
		assert.Len(t, repos, 1)
		assert.Equal(t, repo, repos[0].FullName)
	})

	t.Run("GetMyWatchedRepos", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", "/api/v1/user/subscriptions").
			AddTokenAuth(tokenWithRepoScope)
		resp := MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, "1", resp.Header().Get("X-Total-Count"))

		repos := DecodeJSON(t, resp, []api.Repository{})
		assert.Len(t, repos, 1)
		assert.Equal(t, repo, repos[0].FullName)
	})

	t.Run("IsWatching", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/subscription", repo))
		MakeRequest(t, req, http.StatusUnauthorized)

		req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/subscription", repo)).
			AddTokenAuth(tokenWithRepoScope)
		MakeRequest(t, req, http.StatusOK)

		req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/subscription", repo+"notexisting")).
			AddTokenAuth(tokenWithRepoScope)
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("Unwatch", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/repos/%s/subscription", repo)).
			AddTokenAuth(tokenWithRepoScope)
		MakeRequest(t, req, http.StatusNoContent)
	})
}

func TestAPIWatchPublicOnly(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user1")
	writeRepoToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeReadUser)
	publicOnlyToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopePublicOnly, auth_model.AccessTokenScopeReadUser, auth_model.AccessTokenScopeReadRepository)

	MakeRequest(t, NewRequest(t, "PUT", "/api/v1/repos/user2/repo1/subscription").AddTokenAuth(writeRepoToken), http.StatusOK)
	MakeRequest(t, NewRequest(t, "PUT", "/api/v1/repos/user2/repo2/subscription").AddTokenAuth(writeRepoToken), http.StatusOK)

	resp := MakeRequest(t, NewRequest(t, "GET", "/api/v1/user/subscriptions").AddTokenAuth(publicOnlyToken), http.StatusOK)
	repos := DecodeJSON(t, resp, []api.Repository{})
	for _, r := range repos {
		assert.False(t, r.Private, "private repo %s leaked via /user/subscriptions", r.FullName)
	}
	assert.NotContains(t, repoNames(repos), "user2/repo2")

	resp = MakeRequest(t, NewRequest(t, "GET", "/api/v1/users/user1/subscriptions").AddTokenAuth(publicOnlyToken), http.StatusOK)
	repos = DecodeJSON(t, resp, []api.Repository{})
	for _, r := range repos {
		assert.False(t, r.Private, "private repo %s leaked via /users/{username}/subscriptions", r.FullName)
	}
	assert.NotContains(t, repoNames(repos), "user2/repo2")
}
