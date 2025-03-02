// Copyright 2017 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	org_model "code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	org_service "code.gitea.io/gitea/services/org"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestCreateForkNoLogin(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	req := NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo1/forks", &api.CreateForkOption{})
	MakeRequest(t, req, http.StatusUnauthorized)
}

func TestAPIForkListLimitedAndPrivateRepos(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user1Sess := loginUser(t, "user1")
	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user1"})

	// fork into a limited org
	limitedOrg := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 22})
	assert.EqualValues(t, api.VisibleTypeLimited, limitedOrg.Visibility)

	ownerTeam1, err := org_model.OrgFromUser(limitedOrg).GetOwnerTeam(db.DefaultContext)
	assert.NoError(t, err)
	assert.NoError(t, org_service.AddTeamMember(db.DefaultContext, ownerTeam1, user1))
	user1Token := getTokenForLoggedInUser(t, user1Sess, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteOrganization)
	req := NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo1/forks", &api.CreateForkOption{
		Organization: &limitedOrg.Name,
	}).AddTokenAuth(user1Token)
	MakeRequest(t, req, http.StatusAccepted)

	// fork into a private org
	user4Sess := loginUser(t, "user4")
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user4"})
	privateOrg := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 23})
	assert.EqualValues(t, api.VisibleTypePrivate, privateOrg.Visibility)

	ownerTeam2, err := org_model.OrgFromUser(privateOrg).GetOwnerTeam(db.DefaultContext)
	assert.NoError(t, err)
	assert.NoError(t, org_service.AddTeamMember(db.DefaultContext, ownerTeam2, user4))
	user4Token := getTokenForLoggedInUser(t, user4Sess, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteOrganization)
	req = NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo1/forks", &api.CreateForkOption{
		Organization: &privateOrg.Name,
	}).AddTokenAuth(user4Token)
	MakeRequest(t, req, http.StatusAccepted)

	t.Run("Anonymous", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", "/api/v1/repos/user2/repo1/forks")
		resp := MakeRequest(t, req, http.StatusOK)

		var forks []*api.Repository
		DecodeJSON(t, resp, &forks)

		assert.Empty(t, forks)
		assert.EqualValues(t, "0", resp.Header().Get("X-Total-Count"))
	})

	t.Run("Logged in", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", "/api/v1/repos/user2/repo1/forks").AddTokenAuth(user1Token)
		resp := MakeRequest(t, req, http.StatusOK)

		var forks []*api.Repository
		DecodeJSON(t, resp, &forks)

		assert.Len(t, forks, 2)
		assert.EqualValues(t, "2", resp.Header().Get("X-Total-Count"))

		assert.NoError(t, org_service.AddTeamMember(db.DefaultContext, ownerTeam2, user1))

		req = NewRequest(t, "GET", "/api/v1/repos/user2/repo1/forks").AddTokenAuth(user1Token)
		resp = MakeRequest(t, req, http.StatusOK)

		forks = []*api.Repository{}
		DecodeJSON(t, resp, &forks)

		assert.Len(t, forks, 2)
		assert.EqualValues(t, "2", resp.Header().Get("X-Total-Count"))
	})
}

func TestGetPrivateReposForks(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user1Sess := loginUser(t, "user1")
	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2}) // private repository
	privateOrg := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 23})
	user1Token := getTokenForLoggedInUser(t, user1Sess, auth_model.AccessTokenScopeWriteRepository)

	forkedRepoName := "forked-repo"
	// create fork from a private repository
	req := NewRequestWithJSON(t, "POST", "/api/v1/repos/"+repo2.FullName()+"/forks", &api.CreateForkOption{
		Organization: &privateOrg.Name,
		Name:         &forkedRepoName,
	}).AddTokenAuth(user1Token)
	MakeRequest(t, req, http.StatusAccepted)

	// test get a private fork without clear permissions
	req = NewRequest(t, "GET", "/api/v1/repos/"+repo2.FullName()+"/forks").AddTokenAuth(user1Token)
	resp := MakeRequest(t, req, http.StatusOK)

	forks := []*api.Repository{}
	DecodeJSON(t, resp, &forks)
	assert.Len(t, forks, 1)
	assert.EqualValues(t, "1", resp.Header().Get("X-Total-Count"))
	assert.EqualValues(t, "forked-repo", forks[0].Name)
	assert.EqualValues(t, privateOrg.Name, forks[0].Owner.UserName)
}
