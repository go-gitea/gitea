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
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/test"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIStar(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := "user1"
	repo := "user2/repo1"

	session := loginUser(t, user)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadUser)
	tokenWithUserScope := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteUser, auth_model.AccessTokenScopeWriteRepository)

	t.Run("Star", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "PUT", "/api/v1/user/starred/"+repo).
			AddTokenAuth(tokenWithUserScope)
		MakeRequest(t, req, http.StatusNoContent)

		// blocked user can't star a repo
		user34 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 34})
		req = NewRequest(t, "PUT", "/api/v1/user/starred/"+repo).
			AddTokenAuth(getUserToken(t, user34.Name, auth_model.AccessTokenScopeWriteRepository))
		MakeRequest(t, req, http.StatusForbidden)
	})

	t.Run("GetStarredRepos", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/users/%s/starred", user)).
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, "1", resp.Header().Get("X-Total-Count"))

		repos := DecodeJSON(t, resp, []api.Repository{})
		assert.Len(t, repos, 1)
		assert.Equal(t, repo, repos[0].FullName)
	})

	t.Run("GetMyStarredRepos", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", "/api/v1/user/starred").
			AddTokenAuth(tokenWithUserScope)
		resp := MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, "1", resp.Header().Get("X-Total-Count"))

		repos := DecodeJSON(t, resp, []api.Repository{})
		assert.Len(t, repos, 1)
		assert.Equal(t, repo, repos[0].FullName)
	})

	t.Run("IsStarring", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", "/api/v1/user/starred/"+repo).
			AddTokenAuth(tokenWithUserScope)
		MakeRequest(t, req, http.StatusNoContent)

		req = NewRequest(t, "GET", "/api/v1/user/starred/"+repo+"notexisting").
			AddTokenAuth(tokenWithUserScope)
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("Unstar", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "DELETE", "/api/v1/user/starred/"+repo).
			AddTokenAuth(tokenWithUserScope)
		MakeRequest(t, req, http.StatusNoContent)
	})
}

func TestAPIStarDisabled(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := "user1"
	repo := "user2/repo1"

	session := loginUser(t, user)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadUser)
	tokenWithUserScope := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteUser, auth_model.AccessTokenScopeWriteRepository)

	defer test.MockVariableValue(&setting.Repository.DisableStars, true)()

	t.Run("Star", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "PUT", "/api/v1/user/starred/"+repo).
			AddTokenAuth(tokenWithUserScope)
		MakeRequest(t, req, http.StatusForbidden)

		user34 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 34})
		req = NewRequest(t, "PUT", "/api/v1/user/starred/"+repo).
			AddTokenAuth(getUserToken(t, user34.Name, auth_model.AccessTokenScopeWriteRepository))
		MakeRequest(t, req, http.StatusForbidden)
	})

	t.Run("GetStarredRepos", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/users/%s/starred", user)).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusForbidden)
	})

	t.Run("GetMyStarredRepos", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", "/api/v1/user/starred").
			AddTokenAuth(tokenWithUserScope)
		MakeRequest(t, req, http.StatusForbidden)
	})

	t.Run("IsStarring", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", "/api/v1/user/starred/"+repo).
			AddTokenAuth(tokenWithUserScope)
		MakeRequest(t, req, http.StatusForbidden)

		req = NewRequest(t, "GET", "/api/v1/user/starred/"+repo+"notexisting").
			AddTokenAuth(tokenWithUserScope)
		MakeRequest(t, req, http.StatusForbidden)
	})

	t.Run("Unstar", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "DELETE", "/api/v1/user/starred/"+repo).
			AddTokenAuth(tokenWithUserScope)
		MakeRequest(t, req, http.StatusForbidden)
	})
}

func TestAPIStarPublicOnly(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	token := getUserToken(t, "user2", auth_model.AccessTokenScopeReadUser, auth_model.AccessTokenScopeReadRepository, auth_model.AccessTokenScopePublicOnly)
	req := NewRequest(t, "GET", "/api/v1/user/starred").
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)

	repos := DecodeJSON(t, resp, []api.Repository{})
	if assert.Len(t, repos, 1) {
		assert.Equal(t, "user5/repo4", repos[0].FullName)
	}

	req = NewRequest(t, "GET", "/api/v1/users/user2/starred").
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	repos = DecodeJSON(t, resp, []api.Repository{})
	require.Len(t, repos, 1)
	assert.Equal(t, "user5/repo4", repos[0].FullName)
}
