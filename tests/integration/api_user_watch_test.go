// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

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

		var repos []api.Repository
		DecodeJSON(t, resp, &repos)
		assert.Len(t, repos, 1)
		assert.Equal(t, repo, repos[0].FullName)
	})

	t.Run("GetMyWatchedRepos", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", "/api/v1/user/subscriptions").
			AddTokenAuth(tokenWithRepoScope)
		resp := MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, "1", resp.Header().Get("X-Total-Count"))

		var repos []api.Repository
		DecodeJSON(t, resp, &repos)
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
