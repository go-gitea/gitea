// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIUserReposPublicOnly(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	token := getUserToken(t, "user2", auth_model.AccessTokenScopeReadUser, auth_model.AccessTokenScopeReadRepository, auth_model.AccessTokenScopePublicOnly)
	req := NewRequest(t, "GET", "/api/v1/user/repos").
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)

	var repos []api.Repository
	DecodeJSON(t, resp, &repos)
	assert.NotEmpty(t, repos)
	for _, repo := range repos {
		assert.False(t, repo.Private)
	}
	assert.NotContains(t, repoNames(repos), "user2/repo2")

	req = NewRequest(t, "GET", "/api/v1/users/user2/repos").
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &repos)
	assert.NotEmpty(t, repos)
	for _, repo := range repos {
		assert.False(t, repo.Private)
	}
	assert.NotContains(t, repoNames(repos), "user2/repo2")
}

func repoNames(repos []api.Repository) []string {
	names := make([]string, 0, len(repos))
	for _, repo := range repos {
		names = append(names, repo.FullName)
	}
	return names
}

func TestAPIRepoByIDPublicOnly(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	token := getUserToken(t, "user2", auth_model.AccessTokenScopeReadRepository, auth_model.AccessTokenScopePublicOnly)
	req := NewRequest(t, "GET", "/api/v1/repositories/1").
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK)

	req = NewRequest(t, "GET", "/api/v1/repositories/2").
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusForbidden)
}

func TestAPIActivityFeedsPublicOnly(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	token := getUserToken(t, "user2", auth_model.AccessTokenScopeReadUser)
	req := NewRequest(t, "GET", "/api/v1/users/user2/activities/feeds").
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	var activities []api.Activity
	DecodeJSON(t, resp, &activities)
	assert.NotEmpty(t, activities)

	publicToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadUser, auth_model.AccessTokenScopePublicOnly)
	req = NewRequest(t, "GET", "/api/v1/users/user2/activities/feeds").
		AddTokenAuth(publicToken)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &activities)
	assert.Empty(t, activities)

	orgToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadOrganization)
	req = NewRequest(t, "GET", "/api/v1/orgs/org3/activities/feeds").
		AddTokenAuth(orgToken)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &activities)
	assert.NotEmpty(t, activities)

	publicOrgToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadOrganization, auth_model.AccessTokenScopePublicOnly)
	req = NewRequest(t, "GET", "/api/v1/orgs/org3/activities/feeds").
		AddTokenAuth(publicOrgToken)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &activities)
	assert.Empty(t, activities)
}
