// Copyright 2026 The Gitea Authors. All rights reserved.
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

func TestAPIListMyRepos(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository, auth_model.AccessTokenScopeReadUser)

	// Fetch public repos
	reqPublic := NewRequest(t, "GET", "/api/v1/user/repos?type=public&limit=50").
		AddTokenAuth(token)
	respPublic := MakeRequest(t, reqPublic, http.StatusOK)
	var publicRepos []api.Repository
	DecodeJSON(t, respPublic, &publicRepos)

	// Fetch private repos
	reqPrivate := NewRequest(t, "GET", "/api/v1/user/repos?type=private&limit=50").
		AddTokenAuth(token)
	respPrivate := MakeRequest(t, reqPrivate, http.StatusOK)
	var privateRepos []api.Repository
	DecodeJSON(t, respPrivate, &privateRepos)

	t.Run("TypePublic", func(t *testing.T) {
		assert.NotEmpty(t, publicRepos)
		for _, repo := range publicRepos {
			assert.False(t, repo.Private, "repo %s should be public", repo.Name)
		}
	})

	t.Run("TypePrivate", func(t *testing.T) {
		assert.NotEmpty(t, privateRepos)
		for _, repo := range privateRepos {
			assert.True(t, repo.Private, "repo %s should be private", repo.Name)
		}
	})

	t.Run("NoFilter", func(t *testing.T) {
		req := NewRequest(t, "GET", "/api/v1/user/repos?limit=50").
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)

		var repos []api.Repository
		DecodeJSON(t, resp, &repos)
		// Result should equal sum of public and private
		assert.Len(t, repos, len(publicRepos)+len(privateRepos))
	})

	t.Run("TypeAll", func(t *testing.T) {
		req := NewRequest(t, "GET", "/api/v1/user/repos?type=all&limit=50").
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)

		var repos []api.Repository
		DecodeJSON(t, resp, &repos)
		assert.Len(t, repos, len(publicRepos)+len(privateRepos))
	})

	t.Run("TypeInvalid", func(t *testing.T) {
		req := NewRequest(t, "GET", "/api/v1/user/repos?type=invalid").
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusUnprocessableEntity)
	})
}

func testAPIListReposByType(t *testing.T, urlBase, token string) {
	t.Helper()

	// Fetch public repos
	reqPublic := NewRequest(t, "GET", urlBase+"?type=public&limit=50").
		AddTokenAuth(token)
	respPublic := MakeRequest(t, reqPublic, http.StatusOK)
	var publicRepos []api.Repository
	DecodeJSON(t, respPublic, &publicRepos)

	// Fetch private repos
	reqPrivate := NewRequest(t, "GET", urlBase+"?type=private&limit=50").
		AddTokenAuth(token)
	respPrivate := MakeRequest(t, reqPrivate, http.StatusOK)
	var privateRepos []api.Repository
	DecodeJSON(t, respPrivate, &privateRepos)

	t.Run("TypePublic", func(t *testing.T) {
		assert.NotEmpty(t, publicRepos)
		for _, repo := range publicRepos {
			assert.False(t, repo.Private, "repo %s should be public", repo.Name)
		}
		assert.Equal(t, fmt.Sprintf("%d", len(publicRepos)),
			respPublic.Header().Get("X-Total-Count"),
			"X-Total-Count should match returned public repo count")
	})

	t.Run("TypePrivate", func(t *testing.T) {
		assert.NotEmpty(t, privateRepos)
		for _, repo := range privateRepos {
			assert.True(t, repo.Private, "repo %s should be private", repo.Name)
		}
		assert.Equal(t, fmt.Sprintf("%d", len(privateRepos)),
			respPrivate.Header().Get("X-Total-Count"),
			"X-Total-Count should match returned private repo count")
	})

	t.Run("NoFilter", func(t *testing.T) {
		req := NewRequest(t, "GET", urlBase+"?limit=50").
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		var repos []api.Repository
		DecodeJSON(t, resp, &repos)
		assert.Len(t, repos, len(publicRepos)+len(privateRepos))
	})

	t.Run("TypeAll", func(t *testing.T) {
		req := NewRequest(t, "GET", urlBase+"?type=all&limit=50").
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		var repos []api.Repository
		DecodeJSON(t, resp, &repos)
		assert.Len(t, repos, len(publicRepos)+len(privateRepos))
	})

	t.Run("TypeInvalid", func(t *testing.T) {
		req := NewRequest(t, "GET", urlBase+"?type=invalid").
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusUnprocessableEntity)
	})

	t.Run("TypePrivateUnauthenticated", func(t *testing.T) {
		req := NewRequest(t, "GET", urlBase+"?type=private&limit=50")
		resp := MakeRequest(t, req, http.StatusOK)
		var repos []api.Repository
		DecodeJSON(t, resp, &repos)
		assert.Empty(t, repos, "unauthenticated request should not return private repos")
		assert.Equal(t, "0", resp.Header().Get("X-Total-Count"), "X-Total-Count should not leak private repo count")
	})

	t.Run("TypePrivateNonOwner", func(t *testing.T) {
		// An authenticated user who is not the owner/member must not see private repos
		// in the body, and X-Total-Count must not leak the private repo count either.
		// user8 is not a member of org3 and not a collaborator on user2's private repos.
		nonOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 8})
		nonOwnerSession := loginUser(t, nonOwner.Name)
		nonOwnerToken := getTokenForLoggedInUser(t, nonOwnerSession, auth_model.AccessTokenScopeReadRepository, auth_model.AccessTokenScopeReadUser, auth_model.AccessTokenScopeReadOrganization)
		req := NewRequest(t, "GET", urlBase+"?type=private&limit=50").
			AddTokenAuth(nonOwnerToken)
		resp := MakeRequest(t, req, http.StatusOK)
		var repos []api.Repository
		DecodeJSON(t, resp, &repos)
		assert.Empty(t, repos, "non-owner must not see private repos in body")
		assert.Equal(t, "0", resp.Header().Get("X-Total-Count"),
			"X-Total-Count must not leak private repo count to non-owner")
	})
}

func TestAPIListUserReposByType(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository, auth_model.AccessTokenScopeReadUser)

	testAPIListReposByType(t, fmt.Sprintf("/api/v1/users/%s/repos", user.Name), token)

	t.Run("KnownPublicRepo", func(t *testing.T) {
		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/users/%s/repos?type=public&limit=50", user.Name)).
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		var repos []api.Repository
		DecodeJSON(t, resp, &repos)
		names := make([]string, 0, len(repos))
		for _, r := range repos {
			names = append(names, r.Name)
		}
		assert.Contains(t, names, "repo1", "public repo repo1 must appear in type=public listing")
		assert.NotContains(t, names, "repo2", "private repo repo2 must not appear in type=public listing")
	})

	t.Run("KnownPrivateRepo", func(t *testing.T) {
		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/users/%s/repos?type=private&limit=50", user.Name)).
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		var repos []api.Repository
		DecodeJSON(t, resp, &repos)
		names := make([]string, 0, len(repos))
		for _, r := range repos {
			names = append(names, r.Name)
		}
		assert.Contains(t, names, "repo2", "private repo repo2 must appear in type=private listing")
		assert.NotContains(t, names, "repo1", "public repo repo1 must not appear in type=private listing")
	})
}

func TestAPIListOrgReposByType(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// org3 is owned/accessible by user2 and has both public and private repos
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository, auth_model.AccessTokenScopeReadOrganization)

	org := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})
	testAPIListReposByType(t, fmt.Sprintf("/api/v1/orgs/%s/repos", org.Name), token)

	t.Run("KnownPublicRepo", func(t *testing.T) {
		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/orgs/%s/repos?type=public&limit=50", org.Name)).
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		var repos []api.Repository
		DecodeJSON(t, resp, &repos)
		names := make([]string, 0, len(repos))
		for _, r := range repos {
			names = append(names, r.Name)
		}
		assert.Contains(t, names, "repo21", "public repo repo21 must appear in type=public listing")
		assert.NotContains(t, names, "repo3", "private repo repo3 must not appear in type=public listing")
	})

	t.Run("KnownPrivateRepo", func(t *testing.T) {
		req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/orgs/%s/repos?type=private&limit=50", org.Name)).
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		var repos []api.Repository
		DecodeJSON(t, resp, &repos)
		names := make([]string, 0, len(repos))
		for _, r := range repos {
			names = append(names, r.Name)
		}
		assert.Contains(t, names, "repo3", "private repo repo3 must appear in type=private listing")
		assert.NotContains(t, names, "repo21", "public repo repo21 must not appear in type=private listing")
	})
}
