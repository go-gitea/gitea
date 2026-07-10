// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"strconv"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/organization"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
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

	// user2 is a member of the private org private_org35
	org := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "private_org35"})

	// a full org-scoped token can read the membership permissions
	token := getUserToken(t, "user2", auth_model.AccessTokenScopeReadUser, auth_model.AccessTokenScopeReadOrganization)
	req := NewRequestf(t, "GET", "/api/v1/users/user2/orgs/%s/permissions", org.Name).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK)

	// a public-only token must not disclose permissions for a private org
	publicToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadUser, auth_model.AccessTokenScopeReadOrganization, auth_model.AccessTokenScopePublicOnly)
	req = NewRequestf(t, "GET", "/api/v1/users/user2/orgs/%s/permissions", org.Name).AddTokenAuth(publicToken)
	MakeRequest(t, req, http.StatusNotFound)
}

func TestAPITeamReposPublicOnly(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// team 1 (Owners of org3) has access to the private repos org3/repo3 and org3/repo5
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 1})
	privateRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})
	privateRepo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 5})

	// a full org+repo scoped token sees the private repos
	token := getUserToken(t, "user2", auth_model.AccessTokenScopeReadOrganization, auth_model.AccessTokenScopeReadRepository)
	req := NewRequestf(t, "GET", "/api/v1/teams/%d/repos", team.ID).AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	repos := DecodeJSON(t, resp, []api.Repository{})
	assert.Contains(t, repoNames(repos), privateRepo.FullName())

	// a public-only token must not receive any private repo
	publicToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadOrganization, auth_model.AccessTokenScopeReadRepository, auth_model.AccessTokenScopePublicOnly)
	req = NewRequestf(t, "GET", "/api/v1/teams/%d/repos", team.ID).AddTokenAuth(publicToken)
	resp = MakeRequest(t, req, http.StatusOK)
	repos = DecodeJSON(t, resp, []api.Repository{})
	for _, repo := range repos {
		assert.False(t, repo.Private)
	}
	assert.NotContains(t, repoNames(repos), privateRepo.FullName())
	assert.NotContains(t, repoNames(repos), privateRepo2.FullName())
	// the total-count header must match the filtered page, otherwise it leaks the
	// number of hidden private repos
	assert.Equal(t, strconv.Itoa(len(repos)), resp.Header().Get("X-Total-Count"))

	// the single-repo endpoint must not confirm a private repo for a public-only token
	req = NewRequestf(t, "GET", "/api/v1/teams/%d/repos/%s", team.ID, privateRepo.FullName()).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK)
	req = NewRequestf(t, "GET", "/api/v1/teams/%d/repos/%s", team.ID, privateRepo.FullName()).AddTokenAuth(publicToken)
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
