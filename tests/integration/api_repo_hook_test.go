// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"maps"
	"net/http"
	"strings"
	"testing"

	auth_model "gitea.dev/models/auth"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

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
	// the stored authorization header is a secret and must never be returned by the API
	assert.Empty(t, apiHook.AuthorizationHeader)
	assert.Equal(t, "CI notifications", apiHook.Name)

	// a read-scoped token must not be able to read back the authorization header
	readToken := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadRepository)
	getReq := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/hooks/%d", owner.Name, repo.Name, apiHook.ID)).
		AddTokenAuth(readToken)
	getResp := MakeRequest(t, getReq, http.StatusOK)
	assert.NotContains(t, getResp.Body.String(), "s3cr3t")

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

func TestAPIFluxerHook(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user2")
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	base := "/api/v1/repos/user2/repo1/hooks"
	for _, overrides := range []bool{false, true} {
		config := api.CreateHookOptionConfig{"url": "https://chat.example/api/webhooks/123/token?wait=true", "content_type": "json"}
		if overrides {
			config["username"], config["icon_url"] = "Gitea", "https://gitea.example/icon.png"
		}
		resp := MakeRequest(t, NewRequestWithJSON(t, "POST", base, api.CreateHookOption{Type: "fluxer", Config: config}).AddTokenAuth(token), http.StatusCreated)
		hook := DecodeJSON(t, resp, &api.Hook{})
		assert.Equal(t, "fluxer", hook.Type)
		assert.Equal(t, config["url"], hook.Config["url"])
		assert.Equal(t, config["username"], hook.Config["username"])
		assert.Equal(t, config["icon_url"], hook.Config["icon_url"])
		hookURL := fmt.Sprintf("%s/%d", base, hook.ID)
		get := MakeRequest(t, NewRequest(t, "GET", hookURL).AddTokenAuth(token), http.StatusOK)
		assert.Equal(t, hook.Config, DecodeJSON(t, get, &api.Hook{}).Config)
		for _, change := range []map[string]string{{"username": "Changed"}, {"icon_url": "https://gitea.example/new.png"}, {"username": ""}, {"icon_url": ""}} {
			maps.Copy(hook.Config, change)
			patched := MakeRequest(t, NewRequestWithJSON(t, "PATCH", hookURL, api.EditHookOption{Config: change}).AddTokenAuth(token), http.StatusOK)
			assert.Equal(t, hook.Config, DecodeJSON(t, patched, &api.Hook{}).Config)
		}
		name := "Fluxer hook"
		active := true
		patched := MakeRequest(t, NewRequestWithJSON(t, "PATCH", hookURL, api.EditHookOption{Name: &name, Active: &active}).AddTokenAuth(token), http.StatusOK)
		assert.Equal(t, hook.Config, DecodeJSON(t, patched, &api.Hook{}).Config)
		for _, config := range []map[string]string{{"content_type": "form"}, {"username": strings.Repeat("😀", 41)}, {"icon_url": "invalid"}} {
			MakeRequest(t, NewRequestWithJSON(t, "PATCH", hookURL, api.EditHookOption{Config: config}).AddTokenAuth(token), http.StatusUnprocessableEntity)
		}
		get = MakeRequest(t, NewRequest(t, "GET", hookURL).AddTokenAuth(token), http.StatusOK)
		assert.Equal(t, hook.Config, DecodeJSON(t, get, &api.Hook{}).Config)
	}
	for _, config := range []api.CreateHookOptionConfig{
		{"url": "https://chat.example/webhook", "content_type": "form"},
		{"url": "https://chat.example/webhook", "content_type": "json", "username": strings.Repeat("😀", 41)},
		{"url": "https://chat.example/webhook", "content_type": "json", "icon_url": "invalid"},
	} {
		MakeRequest(t, NewRequestWithJSON(t, "POST", base, api.CreateHookOption{Type: "fluxer", Config: config}).AddTokenAuth(token), http.StatusUnprocessableEntity)
	}
}
