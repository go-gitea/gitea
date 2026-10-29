// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/perm"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/test"
	repo_service "gitea.dev/services/repository"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIStar(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	t.Run("PublicOnly", func(t *testing.T) {
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
	})

	for _, testCase := range []struct {
		name                                   string
		disabled                               bool
		writeStatus, listStatus, missingStatus int
	}{
		{"Enabled", false, http.StatusNoContent, http.StatusOK, http.StatusNotFound},
		{"Disabled", true, http.StatusForbidden, http.StatusForbidden, http.StatusForbidden},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			user := "user1"
			repo := "user2/repo1"

			session := loginUser(t, user)
			token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadUser)
			tokenWithUserScope := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteUser, auth_model.AccessTokenScopeWriteRepository)

			if testCase.disabled {
				defer test.MockVariableValue(&setting.Repository.DisableStars, true)()
			}

			t.Run("Star", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				req := NewRequest(t, "PUT", "/api/v1/user/starred/"+repo).
					AddTokenAuth(tokenWithUserScope)
				MakeRequest(t, req, testCase.writeStatus)

				// blocked user can't star a repo
				user34 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 34})
				req = NewRequest(t, "PUT", "/api/v1/user/starred/"+repo).
					AddTokenAuth(getUserToken(t, user34.Name, auth_model.AccessTokenScopeWriteRepository))
				MakeRequest(t, req, http.StatusForbidden)
			})

			for _, listCase := range []struct {
				name, url, token string
			}{
				{"GetStarredRepos", fmt.Sprintf("/api/v1/users/%s/starred", user), token},
				{"GetMyStarredRepos", "/api/v1/user/starred", tokenWithUserScope},
			} {
				t.Run(listCase.name, func(t *testing.T) {
					defer tests.PrintCurrentTest(t)()

					req := NewRequest(t, "GET", listCase.url).
						AddTokenAuth(listCase.token)
					resp := MakeRequest(t, req, testCase.listStatus)

					if !testCase.disabled {
						assert.Equal(t, "1", resp.Header().Get("X-Total-Count"))

						repos := DecodeJSON(t, resp, []api.Repository{})
						assert.Len(t, repos, 1)
						assert.Equal(t, repo, repos[0].FullName)
					}
				})
			}

			t.Run("IsStarring", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				req := NewRequest(t, "GET", "/api/v1/user/starred/"+repo).
					AddTokenAuth(tokenWithUserScope)
				MakeRequest(t, req, testCase.writeStatus)

				req = NewRequest(t, "GET", "/api/v1/user/starred/"+repo+"notexisting").
					AddTokenAuth(tokenWithUserScope)
				MakeRequest(t, req, testCase.missingStatus)
			})

			t.Run("Unstar", func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()

				req := NewRequest(t, "DELETE", "/api/v1/user/starred/"+repo).
					AddTokenAuth(tokenWithUserScope)
				MakeRequest(t, req, testCase.writeStatus)
			})
		})
	}

	t.Run("AccessRevoked", func(t *testing.T) {
		// user2/repo2 is a private repository owned by user2
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
		owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
		collaborator := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})

		// grant the collaborator read access and star the private repo as them
		require.NoError(t, repo_service.AddOrUpdateCollaborator(t.Context(), repo, collaborator, perm.AccessModeRead))
		require.NoError(t, repo_model.StarRepo(t.Context(), collaborator, repo, true))

		token := getUserToken(t, collaborator.Name, auth_model.AccessTokenScopeReadUser, auth_model.AccessTokenScopeReadRepository)

		// while access is granted, the private repo is visible in the starred list
		req := NewRequest(t, "GET", "/api/v1/user/starred").AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		repos := DecodeJSON(t, resp, []api.Repository{})
		require.Len(t, repos, 1)
		assert.Equal(t, repo.FullName(), repos[0].FullName)

		// revoke access
		require.NoError(t, repo_service.DeleteCollaboration(t.Context(), repo, collaborator))

		// the star record still exists, but the repo (and its metadata) must no longer be returned
		assert.True(t, repo_model.IsStaring(t.Context(), collaborator.ID, repo.ID))
		req = NewRequest(t, "GET", "/api/v1/user/starred").AddTokenAuth(token)
		resp = MakeRequest(t, req, http.StatusOK)
		repos = DecodeJSON(t, resp, []api.Repository{})
		assert.Empty(t, repos)

		// sanity: the owner still sees their own private repo
		ownerToken := getUserToken(t, owner.Name, auth_model.AccessTokenScopeReadUser, auth_model.AccessTokenScopeReadRepository)
		require.NoError(t, repo_model.StarRepo(t.Context(), owner, repo, true))
		req = NewRequest(t, "GET", "/api/v1/user/starred").AddTokenAuth(ownerToken)
		resp = MakeRequest(t, req, http.StatusOK)
		repos = DecodeJSON(t, resp, []api.Repository{})
		fullNames := make([]string, len(repos))
		for i, r := range repos {
			fullNames[i] = r.FullName
		}
		assert.Contains(t, fullNames, repo.FullName())
	})

	t.Run("OmitsInaccessiblePrivateRepos", func(t *testing.T) {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 40})
		privateRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})
		assert.True(t, privateRepo.IsPrivate)
		assert.NoError(t, repo_service.AddOrUpdateCollaborator(t.Context(), privateRepo, user, perm.AccessModeRead))
		assert.NoError(t, repo_model.StarRepo(t.Context(), user, privateRepo, true))
		assert.NoError(t, repo_service.DeleteCollaboration(t.Context(), privateRepo, user))

		token := getUserToken(t, user.Name, auth_model.AccessTokenScopeReadUser, auth_model.AccessTokenScopeReadRepository)
		req := NewRequest(t, "GET", "/api/v1/user/starred").AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)

		repos := DecodeJSON(t, resp, []api.Repository{})
		for _, repo := range repos {
			assert.NotEqual(t, privateRepo.FullName(), repo.FullName)
		}
	})
}
