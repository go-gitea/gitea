// Copyright 2022 The Gitea Authors. All rights reserved.
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
	webhook_model "gitea.dev/models/webhook"
	api "gitea.dev/modules/structs"
	webhook_module "gitea.dev/modules/webhook"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestAPIListAndGetRepoHookDeliveries(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 37})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, "user1")
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	hook := DecodeJSON(t, MakeRequest(t, NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/hooks", owner.Name, repo.Name), api.CreateHookOption{
		Type: "gitea",
		Config: api.CreateHookOptionConfig{
			"content_type": "json",
			"url":          "http://example.com/",
		},
	}).AddTokenAuth(token), http.StatusCreated), &api.Hook{})

	tasks := make([]*webhook_model.HookTask, 0, 4)
	for i := range 4 {
		task, err := webhook_model.CreateHookTask(t.Context(), &webhook_model.HookTask{
			HookID:         hook.ID,
			EventType:      webhook_module.HookEventPush,
			PayloadVersion: 2,
			IsDelivered:    i < 3,
			IsSucceed:      i < 3,
		})
		require.NoError(t, err)
		tasks = append(tasks, task)
	}

	deliveriesURL := fmt.Sprintf("/api/v1/repos/%s/%s/hooks/%d/deliveries", owner.Name, repo.Name, hook.ID)
	deliveries := DecodeJSON(t, MakeRequest(t, NewRequest(t, "GET", deliveriesURL).AddTokenAuth(token), http.StatusOK), &api.HookDeliveryList{})
	require.Len(t, *deliveries, 4)
	assert.Equal(t, tasks[3].UUID, (*deliveries)[0].UUID)
	assert.False(t, (*deliveries)[0].IsDelivered)
	assert.Nil(t, (*deliveries)[0].Response)
	assert.Nil(t, (*deliveries)[0].Delivered)

	for page, expectedUUIDs := range [][]string{{tasks[3].UUID, tasks[2].UUID}, {tasks[1].UUID, tasks[0].UUID}} {
		t.Run(fmt.Sprintf("page_%d", page+1), func(t *testing.T) {
			resp := MakeRequest(t, NewRequest(t, "GET", fmt.Sprintf("%s?limit=2&page=%d", deliveriesURL, page+1)).AddTokenAuth(token), http.StatusOK)
			deliveries := DecodeJSON(t, resp, &api.HookDeliveryList{})
			require.Len(t, *deliveries, len(expectedUUIDs))
			assert.Equal(t, "4", resp.Header().Get("X-Total-Count"))
			for i, uuid := range expectedUUIDs {
				assert.Equal(t, uuid, (*deliveries)[i].UUID)
			}
		})
	}

	delivery := DecodeJSON(t, MakeRequest(t, NewRequest(t, "GET", deliveriesURL+"/"+tasks[0].UUID).AddTokenAuth(token), http.StatusOK), &api.HookDelivery{})
	assert.Equal(t, tasks[0].UUID, delivery.UUID)
	assert.True(t, delivery.IsDelivered)
	assert.True(t, delivery.IsSucceeded)
	assert.NotNil(t, delivery.Delivered)

	MakeRequest(t, NewRequest(t, "GET", deliveriesURL+"/does-not-exist").AddTokenAuth(token), http.StatusNotFound)

	otherRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	otherOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: otherRepo.OwnerID})
	for _, suffix := range []string{"", "/" + tasks[0].UUID} {
		MakeRequest(t, NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/hooks/%d/deliveries%s", otherOwner.Name, otherRepo.Name, hook.ID, suffix)).
			AddTokenAuth(token), http.StatusNotFound)
	}
}
