// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/test"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIGitHooks(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&setting.DisableGitHooks, false)()

	const testHookContent = `#!/bin/bash
echo "TestGitHookScript"
`

	// user1 is an admin user
	adminSession := loginUser(t, "user1")
	adminReadToken := getTokenForLoggedInUser(t, adminSession, auth_model.AccessTokenScopeReadRepository)
	adminWriteToken := getTokenForLoggedInUser(t, adminSession, auth_model.AccessTokenScopeWriteRepository)
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	owner1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo1.OwnerID})
	ownerSession := loginUser(t, owner1.Name)
	ownerReadToken := getTokenForLoggedInUser(t, ownerSession, auth_model.AccessTokenScopeReadRepository)
	ownerWriteToken := getTokenForLoggedInUser(t, ownerSession, auth_model.AccessTokenScopeWriteRepository)

	for _, testCase := range []struct {
		name   string
		repoID int64
	}{
		{"ListGitHooks", 37},
		{"NoGitHooks", 1},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: testCase.repoID})
			owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

			req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/hooks/git", owner.Name, repo.Name).
				AddTokenAuth(adminReadToken)
			resp := MakeRequest(t, req, http.StatusOK)
			apiGitHooks := DecodeJSON(t, resp, []*api.GitHook{})
			assert.Len(t, apiGitHooks, 3)
			for _, apiGitHook := range apiGitHooks {
				if testCase.repoID == 37 && apiGitHook.Name == "pre-receive" {
					assert.True(t, apiGitHook.IsActive)
					assert.Equal(t, testHookContent, apiGitHook.Content)
				} else {
					assert.False(t, apiGitHook.IsActive)
					assert.Empty(t, apiGitHook.Content)
				}
			}
		})
	}

	t.Run("ListGitHooksNoAccess", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/hooks/git", owner1.Name, repo1.Name).
			AddTokenAuth(ownerReadToken)
		MakeRequest(t, req, http.StatusForbidden)
	})

	t.Run("GetGitHook", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 37})
		owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

		req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/hooks/git/pre-receive", owner.Name, repo.Name).
			AddTokenAuth(adminReadToken)
		resp := MakeRequest(t, req, http.StatusOK)
		apiGitHook := DecodeJSON(t, resp, &api.GitHook{})
		assert.True(t, apiGitHook.IsActive)
		assert.Equal(t, testHookContent, apiGitHook.Content)
	})
	t.Run("GetGitHookNoAccess", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/hooks/git/pre-receive", owner1.Name, repo1.Name).
			AddTokenAuth(ownerReadToken)
		MakeRequest(t, req, http.StatusForbidden)
	})

	t.Run("EditGitHook", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

		urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/hooks/git/pre-receive",
			owner.Name, repo.Name)
		req := NewRequestWithJSON(t, "PATCH", urlStr, &api.EditGitHookOption{
			Content: testHookContent,
		}).AddTokenAuth(adminWriteToken)
		resp := MakeRequest(t, req, http.StatusOK)
		apiGitHook := DecodeJSON(t, resp, &api.GitHook{})
		assert.True(t, apiGitHook.IsActive)
		assert.Equal(t, testHookContent, apiGitHook.Content)

		req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/hooks/git/pre-receive", owner.Name, repo.Name).
			AddTokenAuth(adminWriteToken)
		resp = MakeRequest(t, req, http.StatusOK)
		apiGitHook2 := DecodeJSON(t, resp, &api.GitHook{})
		assert.True(t, apiGitHook2.IsActive)
		assert.Equal(t, testHookContent, apiGitHook2.Content)
	})

	t.Run("EditGitHookNoAccess", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/hooks/git/pre-receive", owner1.Name, repo1.Name)
		req := NewRequestWithJSON(t, "PATCH", urlStr, &api.EditGitHookOption{
			Content: testHookContent,
		}).AddTokenAuth(ownerWriteToken)
		MakeRequest(t, req, http.StatusForbidden)
	})

	t.Run("DeleteGitHook", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 37})
		owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

		req := NewRequestf(t, "DELETE", "/api/v1/repos/%s/%s/hooks/git/pre-receive", owner.Name, repo.Name).
			AddTokenAuth(adminWriteToken)
		MakeRequest(t, req, http.StatusNoContent)

		req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/hooks/git/pre-receive", owner.Name, repo.Name).
			AddTokenAuth(adminWriteToken)
		resp := MakeRequest(t, req, http.StatusOK)
		apiGitHook2 := DecodeJSON(t, resp, &api.GitHook{})
		assert.False(t, apiGitHook2.IsActive)
		assert.Empty(t, apiGitHook2.Content)
	})

	t.Run("DeleteGitHookNoAccess", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequestf(t, "DELETE", "/api/v1/repos/%s/%s/hooks/git/pre-receive", owner1.Name, repo1.Name).
			AddTokenAuth(ownerWriteToken)
		MakeRequest(t, req, http.StatusForbidden)
	})
}
