// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIGitHooks(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&setting.DisableGitHooks, false)()

	const testHookContent = `#!/bin/bash
echo "TestGitHookScript"
`

	t.Run("ListGitHooks", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 37})
		owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

		// user1 is an admin user
		session := loginUser(t, "user1")
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/hooks/git", owner.Name, repo.Name).
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		var apiGitHooks []*api.GitHook
		DecodeJSON(t, resp, &apiGitHooks)
		assert.Len(t, apiGitHooks, 3)
		for _, apiGitHook := range apiGitHooks {
			if apiGitHook.Name == "pre-receive" {
				assert.True(t, apiGitHook.IsActive)
				assert.Equal(t, testHookContent, apiGitHook.Content)
			} else {
				assert.False(t, apiGitHook.IsActive)
				assert.Empty(t, apiGitHook.Content)
			}
		}
	})

	t.Run("NoGitHooks", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

		// user1 is an admin user
		session := loginUser(t, "user1")
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/hooks/git", owner.Name, repo.Name).
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		var apiGitHooks []*api.GitHook
		DecodeJSON(t, resp, &apiGitHooks)
		assert.Len(t, apiGitHooks, 3)
		for _, apiGitHook := range apiGitHooks {
			assert.False(t, apiGitHook.IsActive)
			assert.Empty(t, apiGitHook.Content)
		}
	})

	t.Run("ListGitHooksNoAccess", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

		session := loginUser(t, owner.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/hooks/git", owner.Name, repo.Name).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusForbidden)
	})

	t.Run("GetGitHook", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 37})
		owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

		// user1 is an admin user
		session := loginUser(t, "user1")
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/hooks/git/pre-receive", owner.Name, repo.Name).
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		var apiGitHook *api.GitHook
		DecodeJSON(t, resp, &apiGitHook)
		assert.True(t, apiGitHook.IsActive)
		assert.Equal(t, testHookContent, apiGitHook.Content)
	})
	t.Run("GetGitHookNoAccess", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

		session := loginUser(t, owner.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)
		req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/hooks/git/pre-receive", owner.Name, repo.Name).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusForbidden)
	})

	t.Run("EditGitHook", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

		// user1 is an admin user
		session := loginUser(t, "user1")
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/hooks/git/pre-receive",
			owner.Name, repo.Name)
		req := NewRequestWithJSON(t, "PATCH", urlStr, &api.EditGitHookOption{
			Content: testHookContent,
		}).AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		var apiGitHook *api.GitHook
		DecodeJSON(t, resp, &apiGitHook)
		assert.True(t, apiGitHook.IsActive)
		assert.Equal(t, testHookContent, apiGitHook.Content)

		req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/hooks/git/pre-receive", owner.Name, repo.Name).
			AddTokenAuth(token)
		resp = MakeRequest(t, req, http.StatusOK)
		var apiGitHook2 *api.GitHook
		DecodeJSON(t, resp, &apiGitHook2)
		assert.True(t, apiGitHook2.IsActive)
		assert.Equal(t, testHookContent, apiGitHook2.Content)
	})

	t.Run("EditGitHookNoAccess", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

		session := loginUser(t, owner.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
		urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/hooks/git/pre-receive", owner.Name, repo.Name)
		req := NewRequestWithJSON(t, "PATCH", urlStr, &api.EditGitHookOption{
			Content: testHookContent,
		}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusForbidden)
	})

	t.Run("DeleteGitHook", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 37})
		owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

		// user1 is an admin user
		session := loginUser(t, "user1")
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

		req := NewRequestf(t, "DELETE", "/api/v1/repos/%s/%s/hooks/git/pre-receive", owner.Name, repo.Name).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusNoContent)

		req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/hooks/git/pre-receive", owner.Name, repo.Name).
			AddTokenAuth(token)
		resp := MakeRequest(t, req, http.StatusOK)
		var apiGitHook2 *api.GitHook
		DecodeJSON(t, resp, &apiGitHook2)
		assert.False(t, apiGitHook2.IsActive)
		assert.Empty(t, apiGitHook2.Content)
	})

	t.Run("DeleteGitHookNoAccess", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

		session := loginUser(t, owner.Name)
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
		req := NewRequestf(t, "DELETE", "/api/v1/repos/%s/%s/hooks/git/pre-receive", owner.Name, repo.Name).
			AddTokenAuth(token)
		MakeRequest(t, req, http.StatusForbidden)
	})
}
