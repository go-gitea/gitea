// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

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
	MakeRequest(t, req, http.StatusNotFound)
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
	assertPublicActivitiesOnly(t, activities)

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
	assertPublicActivitiesOnly(t, activities)
}

func TestAPIOrgPermissionsPublicOnly(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// user2 is a member of the private org "private_org35". A full org-scoped token
	// can read the membership permissions...
	token := getUserToken(t, "user2", auth_model.AccessTokenScopeReadUser, auth_model.AccessTokenScopeReadOrganization)
	req := NewRequest(t, "GET", "/api/v1/users/user2/orgs/private_org35/permissions").
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK)

	// ...but a public-only token must not disclose permissions for a private org.
	publicToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadUser, auth_model.AccessTokenScopeReadOrganization, auth_model.AccessTokenScopePublicOnly)
	req = NewRequest(t, "GET", "/api/v1/users/user2/orgs/private_org35/permissions").
		AddTokenAuth(publicToken)
	MakeRequest(t, req, http.StatusNotFound)
}

func TestAPITeamReposPublicOnly(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// team 1 (Owners) of the public org "org3" grants access to private repos
	// (org3/repo3, org3/repo5). A full org-scoped token sees them...
	token := getUserToken(t, "user2", auth_model.AccessTokenScopeReadOrganization, auth_model.AccessTokenScopeReadRepository)
	req := NewRequest(t, "GET", "/api/v1/teams/1/repos").
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	var repos []api.Repository
	DecodeJSON(t, resp, &repos)
	assert.Contains(t, repoNames(repos), "org3/repo3")

	// ...but a public-only token must not receive any private repo metadata.
	publicToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadOrganization, auth_model.AccessTokenScopeReadRepository, auth_model.AccessTokenScopePublicOnly)
	req = NewRequest(t, "GET", "/api/v1/teams/1/repos").
		AddTokenAuth(publicToken)
	resp = MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &repos)
	for _, repo := range repos {
		assert.False(t, repo.Private)
	}
	assert.NotContains(t, repoNames(repos), "org3/repo3")
	assert.NotContains(t, repoNames(repos), "org3/repo5")

	// the single-repo endpoint must not confirm a private repo for a public-only token
	req = NewRequest(t, "GET", "/api/v1/teams/1/repos/org3/repo3").
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK)
	req = NewRequest(t, "GET", "/api/v1/teams/1/repos/org3/repo3").
		AddTokenAuth(publicToken)
	MakeRequest(t, req, http.StatusNotFound)
}

func assertPublicActivitiesOnly(t *testing.T, activities []api.Activity) {
	t.Helper()

	for _, activity := range activities {
		assert.False(t, activity.IsPrivate)
		if activity.Repo != nil {
			assert.False(t, activity.Repo.Private)
		}
	}
}
