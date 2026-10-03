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
	createReq := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/hooks", owner.Name, repo.Name), api.CreateHookOption{
		Type: "gitea",
		Config: api.CreateHookOptionConfig{
			"content_type": "json",
			"url":          "http://example.com/",
		},
	}).AddTokenAuth(token)
	createResp := MakeRequest(t, createReq, http.StatusCreated)
	hook := DecodeJSON(t, createResp, &api.Hook{})

	deliveredTask, err := webhook_model.CreateHookTask(t.Context(), &webhook_model.HookTask{
		HookID:         hook.ID,
		PayloadContent: `{"foo":"bar"}`,
		EventType:      webhook_module.HookEventPush,
		PayloadVersion: 2,
		IsDelivered:    true,
		IsSucceed:      true,
	})
	assert.NoError(t, err)
	_, err = webhook_model.CreateHookTask(t.Context(), &webhook_model.HookTask{
		HookID:         hook.ID,
		PayloadContent: `{"foo":"baz"}`,
		EventType:      webhook_module.HookEventPush,
		PayloadVersion: 2,
	})
	assert.NoError(t, err)

	listReq := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/hooks/%d/deliveries", owner.Name, repo.Name, hook.ID)).
		AddTokenAuth(token)
	listResp := MakeRequest(t, listReq, http.StatusOK)
	deliveries := DecodeJSON(t, listResp, &api.HookDeliveryList{})
	assert.Len(t, *deliveries, 2)

	getReq := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/hooks/%d/deliveries/%s", owner.Name, repo.Name, hook.ID, deliveredTask.UUID)).
		AddTokenAuth(token)
	getResp := MakeRequest(t, getReq, http.StatusOK)
	delivery := DecodeJSON(t, getResp, &api.HookDelivery{})
	assert.Equal(t, deliveredTask.UUID, delivery.UUID)
	assert.True(t, delivery.IsDelivered)
	assert.True(t, delivery.IsSucceeded)
	assert.NotNil(t, delivery.Delivered)

	notFoundReq := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/hooks/%d/deliveries/does-not-exist", owner.Name, repo.Name, hook.ID)).
		AddTokenAuth(token)
	MakeRequest(t, notFoundReq, http.StatusNotFound)

	// an undelivered task has no response recorded yet
	undeliveredReq := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/hooks/%d/deliveries", owner.Name, repo.Name, hook.ID)).
		AddTokenAuth(token)
	undeliveredResp := MakeRequest(t, undeliveredReq, http.StatusOK)
	listed := DecodeJSON(t, undeliveredResp, &api.HookDeliveryList{})
	var pending *api.HookDelivery
	for _, d := range *listed {
		if !d.IsDelivered {
			pending = d
		}
	}
	if assert.NotNil(t, pending) {
		assert.Nil(t, pending.Response)
		assert.Nil(t, pending.Delivered)
	}

	// a hook ID that belongs to a different repository must not be reachable through this repo's route
	otherRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	otherOwner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: otherRepo.OwnerID})
	crossRepoReq := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/hooks/%d/deliveries", otherOwner.Name, otherRepo.Name, hook.ID)).
		AddTokenAuth(token)
	MakeRequest(t, crossRepoReq, http.StatusNotFound)

	crossRepoGetReq := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/hooks/%d/deliveries/%s", otherOwner.Name, otherRepo.Name, hook.ID, deliveredTask.UUID)).
		AddTokenAuth(token)
	MakeRequest(t, crossRepoGetReq, http.StatusNotFound)
}

func TestAPIListRepoHookDeliveriesPagination(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 37})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	session := loginUser(t, "user1")
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	createReq := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/hooks", owner.Name, repo.Name), api.CreateHookOption{
		Type: "gitea",
		Config: api.CreateHookOptionConfig{
			"content_type": "json",
			"url":          "http://example.com/",
		},
	}).AddTokenAuth(token)
	createResp := MakeRequest(t, createReq, http.StatusCreated)
	hook := DecodeJSON(t, createResp, &api.Hook{})

	const total = 5
	uuids := make([]string, 0, total)
	for i := range total {
		task, err := webhook_model.CreateHookTask(t.Context(), &webhook_model.HookTask{
			HookID:         hook.ID,
			PayloadContent: fmt.Sprintf(`{"i":%d}`, i),
			EventType:      webhook_module.HookEventPush,
			PayloadVersion: 2,
			IsDelivered:    true,
			IsSucceed:      true,
		})
		assert.NoError(t, err)
		uuids = append(uuids, task.UUID)
	}

	pageOneReq := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/hooks/%d/deliveries?limit=2&page=1", owner.Name, repo.Name, hook.ID)).
		AddTokenAuth(token)
	pageOneResp := MakeRequest(t, pageOneReq, http.StatusOK)
	pageOne := DecodeJSON(t, pageOneResp, &api.HookDeliveryList{})
	assert.Len(t, *pageOne, 2)
	assert.Equal(t, "5", pageOneResp.Header().Get("X-Total-Count"))
	// most recent deliveries come first
	assert.Equal(t, uuids[total-1], (*pageOne)[0].UUID)
	assert.Equal(t, uuids[total-2], (*pageOne)[1].UUID)

	pageTwoReq := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/hooks/%d/deliveries?limit=2&page=2", owner.Name, repo.Name, hook.ID)).
		AddTokenAuth(token)
	pageTwoResp := MakeRequest(t, pageTwoReq, http.StatusOK)
	pageTwo := DecodeJSON(t, pageTwoResp, &api.HookDeliveryList{})
	assert.Len(t, *pageTwo, 2)
	assert.Equal(t, uuids[total-3], (*pageTwo)[0].UUID)
	assert.Equal(t, uuids[total-4], (*pageTwo)[1].UUID)

	// no overlap between pages
	assert.NotEqual(t, (*pageOne)[0].UUID, (*pageTwo)[0].UUID)
	assert.NotEqual(t, (*pageOne)[1].UUID, (*pageTwo)[0].UUID)
}
