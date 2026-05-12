// Copyright 2022 The Gitea Authors. All rights reserved.
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
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPICreateHook(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 37})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	// user1 is an admin user
	session := loginUser(t, "user1")
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/%s", owner.Name, repo.Name, "hooks"), api.CreateHookOption{
		Type: "gitea",
		Config: api.CreateHookOptionConfig{
			"content_type": "json",
			"url":          "http://example.com/",
		},
		AuthorizationHeader: "Bearer s3cr3t",
		Name:                "  CI notifications  ",
	}).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusCreated)

	apiHook := DecodeJSON(t, resp, &api.Hook{})
	assert.Equal(t, "http://example.com/", apiHook.Config["url"])
	assert.Equal(t, "Bearer s3cr3t", apiHook.AuthorizationHeader)
	assert.Equal(t, "CI notifications", apiHook.Name)

	newName := "Deploy hook"
	patchReq := NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/repos/%s/%s/hooks/%d", owner.Name, repo.Name, apiHook.ID), api.EditHookOption{
		Name: &newName,
	}).AddTokenAuth(token)
	patchResp := MakeRequest(t, patchReq, http.StatusOK)
	patched := DecodeJSON(t, patchResp, &api.Hook{})
	assert.Equal(t, newName, patched.Name)

	hooksURL := fmt.Sprintf("/api/v1/repos/%s/%s/hooks", owner.Name, repo.Name)

	// Create with Name field omitted: Name should be ""
	req2 := NewRequestWithJSON(t, "POST", hooksURL, api.CreateHookOption{
		Type: "gitea",
		Config: api.CreateHookOptionConfig{
			"content_type": "json",
			"url":          "http://example.com/",
		},
	}).AddTokenAuth(token)
	resp2 := MakeRequest(t, req2, http.StatusCreated)
	created := DecodeJSON(t, resp2, &api.Hook{})
	assert.Empty(t, created.Name)

	hookURL := fmt.Sprintf("/api/v1/repos/%s/%s/hooks/%d", owner.Name, repo.Name, created.ID)

	// PATCH with Name set: existing Name must be updated
	setName := "original"
	setReq := NewRequestWithJSON(t, "PATCH", hookURL, api.EditHookOption{
		Name: &setName,
	}).AddTokenAuth(token)
	MakeRequest(t, setReq, http.StatusOK)

	// PATCH without Name field: name must remain "original"
	patchReq2 := NewRequestWithJSON(t, "PATCH", hookURL, api.EditHookOption{}).AddTokenAuth(token)
	patchResp2 := MakeRequest(t, patchReq2, http.StatusOK)
	notCleared := DecodeJSON(t, patchResp2, &api.Hook{})
	assert.Equal(t, "original", notCleared.Name)

	// PATCH with Name: "" explicitly: Name should be cleared to ""
	clearReq := NewRequestWithJSON(t, "PATCH", hookURL, api.EditHookOption{
		Name: new(""),
	}).AddTokenAuth(token)
	clearResp := MakeRequest(t, clearReq, http.StatusOK)
	cleared := DecodeJSON(t, clearResp, &api.Hook{})
	assert.Empty(t, cleared.Name)
}
