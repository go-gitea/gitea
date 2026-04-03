// Copyright 2017 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	org_model "code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	org_service "code.gitea.io/gitea/services/org"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIFork(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	t.Run("CreateForkNoLogin", testCreateForkNoLogin)
	t.Run("CreateForkOrgNoCreatePermission", testCreateForkOrgNoCreatePermission)
	t.Run("APIForkListLimitedAndPrivateRepos", testAPIForkListLimitedAndPrivateRepos)
	t.Run("GetPrivateReposForks", testGetPrivateReposForks)
}

func testCreateForkNoLogin(t *testing.T) {
	req := NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo1/forks", &api.CreateForkOption{})
	MakeRequest(t, req, http.StatusUnauthorized)
}

func testCreateForkOrgNoCreatePermission(t *testing.T) {
	user4Sess := loginUser(t, "user4")
	org := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})

	canCreate, err := org_model.OrgFromUser(org).CanCreateOrgRepo(t.Context(), 4)
	assert.NoError(t, err)
	assert.False(t, canCreate)

	user4Token := getTokenForLoggedInUser(t, user4Sess, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteOrganization)
	req := NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo1/forks", &api.CreateForkOption{
		Organization: &org.Name,
	}).AddTokenAuth(user4Token)
	MakeRequest(t, req, http.StatusForbidden)
}

func testAPIForkListLimitedAndPrivateRepos(t *testing.T) {
	user1Sess := loginUser(t, "user1")
	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user1"})

	// fork into a limited org
	limitedOrg := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 22})
	assert.Equal(t, api.VisibleTypeLimited, limitedOrg.Visibility)

	ownerTeam1, err := org_model.OrgFromUser(limitedOrg).GetOwnerTeam(t.Context())
	assert.NoError(t, err)
	assert.NoError(t, org_service.AddTeamMember(t.Context(), ownerTeam1, user1))
	user1Token := getTokenForLoggedInUser(t, user1Sess, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteOrganization)
	req := NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo1/forks", &api.CreateForkOption{
		Organization: &limitedOrg.Name,
	}).AddTokenAuth(user1Token)
	MakeRequest(t, req, http.StatusAccepted)

	// fork into a private org
	user4Sess := loginUser(t, "user4")
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user4"})
	privateOrg := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 23})
	assert.Equal(t, api.VisibleTypePrivate, privateOrg.Visibility)

	ownerTeam2, err := org_model.OrgFromUser(privateOrg).GetOwnerTeam(t.Context())
	assert.NoError(t, err)
	assert.NoError(t, org_service.AddTeamMember(t.Context(), ownerTeam2, user4))
	user4Token := getTokenForLoggedInUser(t, user4Sess, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteOrganization)
	req = NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo1/forks", &api.CreateForkOption{
		Organization: &privateOrg.Name,
	}).AddTokenAuth(user4Token)
	MakeRequest(t, req, http.StatusAccepted)

	t.Run("Anonymous", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", "/api/v1/repos/user2/repo1/forks")
		resp := MakeRequest(t, req, http.StatusOK)
		forks := DecodeJSON(t, resp, []*api.Repository{})
		assert.Empty(t, forks)
		assert.Equal(t, "0", resp.Header().Get("X-Total-Count"))
	})

	t.Run("Logged in", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", "/api/v1/repos/user2/repo1/forks").AddTokenAuth(user1Token)
		resp := MakeRequest(t, req, http.StatusOK)

		forks := DecodeJSON(t, resp, []*api.Repository{})
		assert.Len(t, forks, 2)
		assert.Equal(t, "2", resp.Header().Get("X-Total-Count"))

		assert.NoError(t, org_service.AddTeamMember(t.Context(), ownerTeam2, user1))

		req = NewRequest(t, "GET", "/api/v1/repos/user2/repo1/forks").AddTokenAuth(user1Token)
		resp = MakeRequest(t, req, http.StatusOK)
		forks = DecodeJSON(t, resp, []*api.Repository{})
		assert.Len(t, forks, 2)
		assert.Equal(t, "2", resp.Header().Get("X-Total-Count"))
	})
}

func testGetPrivateReposForks(t *testing.T) {
	user1Sess := loginUser(t, "user1")
	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2}) // private repository
	privateOrg := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 23})
	user1Token := getTokenForLoggedInUser(t, user1Sess, auth_model.AccessTokenScopeWriteRepository)

	// create fork from a private repository
	req := NewRequestWithJSON(t, "POST", "/api/v1/repos/"+repo2.FullName()+"/forks", &api.CreateForkOption{
		Organization: &privateOrg.Name,
		Name:         new("forked-repo"),
	}).AddTokenAuth(user1Token)
	MakeRequest(t, req, http.StatusAccepted)

	// test get a private fork without clear permissions
	req = NewRequest(t, "GET", "/api/v1/repos/"+repo2.FullName()+"/forks").AddTokenAuth(user1Token)
	resp := MakeRequest(t, req, http.StatusOK)

	forks := DecodeJSON(t, resp, []*api.Repository{})
	assert.Len(t, forks, 1)
	assert.Equal(t, "1", resp.Header().Get("X-Total-Count"))
	assert.Equal(t, "forked-repo", forks[0].Name)
	assert.Equal(t, privateOrg.Name, forks[0].Owner.UserName)
}
