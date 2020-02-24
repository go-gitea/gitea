// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

const testHookContent = `#!/bin/bash

echo Hello, World!
`

func TestAPIListGitHooks(t *testing.T) {
	defer prepareTestEnv(t)()

	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 37}).(*models.Repository)
	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	// user1 is an admin user
	session := loginUser(t, "user1")
	token := getTokenForLoggedInUser(t, session)
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/hooks/git?token=%s",
		owner.Name, repo.Name, token)
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
}

func TestAPIListGitHooksNoHooks(t *testing.T) {
	defer prepareTestEnv(t)()

	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	// user1 is an admin user
	session := loginUser(t, "user1")
	token := getTokenForLoggedInUser(t, session)
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/hooks/git?token=%s",
		owner.Name, repo.Name, token)
	resp := MakeRequest(t, req, http.StatusOK)
	var apiGitHooks []*api.GitHook
	DecodeJSON(t, resp, &apiGitHooks)
	assert.Len(t, apiGitHooks, 3)
	for _, apiGitHook := range apiGitHooks {
		assert.False(t, apiGitHook.IsActive)
		assert.Empty(t, apiGitHook.Content)
	}
}

func TestAPIListGitHooksNoAccess(t *testing.T) {
	defer prepareTestEnv(t)()

	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session)
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/hooks/git?token=%s",
		owner.Name, repo.Name, token)
	MakeRequest(t, req, http.StatusForbidden)
}

func TestAPIGetGitHook(t *testing.T) {
	defer prepareTestEnv(t)()

	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 37}).(*models.Repository)
	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	// user1 is an admin user
	session := loginUser(t, "user1")
	token := getTokenForLoggedInUser(t, session)
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/hooks/git/pre-receive?token=%s",
		owner.Name, repo.Name, token)
	resp := MakeRequest(t, req, http.StatusOK)
	var apiGitHook *api.GitHook
	DecodeJSON(t, resp, &apiGitHook)
	assert.True(t, apiGitHook.IsActive)
	assert.Equal(t, testHookContent, apiGitHook.Content)
}

func TestAPIGetGitHookNoAccess(t *testing.T) {
	defer prepareTestEnv(t)()

	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session)
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/hooks/git/pre-receive?token=%s",
		owner.Name, repo.Name, token)
	MakeRequest(t, req, http.StatusForbidden)
}

func TestAPIEditGitHook(t *testing.T) {
	defer prepareTestEnv(t)()

	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	// user1 is an admin user
	session := loginUser(t, "user1")
	token := getTokenForLoggedInUser(t, session)

	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/hooks/git/pre-receive?token=%s",
		owner.Name, repo.Name, token)
	req := NewRequestWithJSON(t, "PATCH", urlStr, &api.EditGitHookOption{
		Content: testHookContent,
	})
	resp := MakeRequest(t, req, http.StatusOK)
	var apiGitHook *api.GitHook
	DecodeJSON(t, resp, &apiGitHook)
	assert.True(t, apiGitHook.IsActive)
	assert.Equal(t, testHookContent, apiGitHook.Content)

	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/hooks/git/pre-receive?token=%s",
		owner.Name, repo.Name, token)
	resp = MakeRequest(t, req, http.StatusOK)
	var apiGitHook2 *api.GitHook
	DecodeJSON(t, resp, &apiGitHook2)
	assert.True(t, apiGitHook2.IsActive)
	assert.Equal(t, testHookContent, apiGitHook2.Content)
}

func TestAPIEditGitHookNoAccess(t *testing.T) {
	defer prepareTestEnv(t)()

	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/hooks/git/pre-receive?token=%s",
		owner.Name, repo.Name, token)
	req := NewRequestWithJSON(t, "PATCH", urlStr, &api.EditGitHookOption{
		Content: testHookContent,
	})
	MakeRequest(t, req, http.StatusForbidden)
}

func TestAPIDeleteGitHook(t *testing.T) {
	defer prepareTestEnv(t)()

	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 37}).(*models.Repository)
	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	// user1 is an admin user
	session := loginUser(t, "user1")
	token := getTokenForLoggedInUser(t, session)

	req := NewRequestf(t, "DELETE", "/api/v1/repos/%s/%s/hooks/git/pre-receive?token=%s",
		owner.Name, repo.Name, token)
	MakeRequest(t, req, http.StatusNoContent)

	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/hooks/git/pre-receive?token=%s",
		owner.Name, repo.Name, token)
	resp := MakeRequest(t, req, http.StatusOK)
	var apiGitHook2 *api.GitHook
	DecodeJSON(t, resp, &apiGitHook2)
	assert.False(t, apiGitHook2.IsActive)
	assert.Empty(t, apiGitHook2.Content)
}

func TestAPIDeleteGitHookNoAccess(t *testing.T) {
	defer prepareTestEnv(t)()

	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session)
	req := NewRequestf(t, "DELETE", "/api/v1/repos/%s/%s/hooks/git/pre-receive?token=%s",
		owner.Name, repo.Name, token)
	MakeRequest(t, req, http.StatusForbidden)
}
