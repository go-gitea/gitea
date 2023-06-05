// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	org_model "code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	unit_model "code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIOrgCreate(t *testing.T) {
	onGiteaRun(t, func(*testing.T, *url.URL) {
		token := getUserToken(t, "user1", auth_model.AccessTokenScopeWriteOrganization)

		org := api.CreateOrgOption{
			UserName:    "user1_org",
			FullName:    "User1's organization",
			Description: "This organization created by user1",
			Website:     "https://try.gitea.io",
			Location:    "Shanghai",
			Visibility:  "limited",
		}
		req := NewRequestWithJSON(t, "POST", "/api/v1/orgs?token="+token, &org)
		resp := MakeRequest(t, req, http.StatusCreated)

		var apiOrg api.Organization
		DecodeJSON(t, resp, &apiOrg)

		assert.Equal(t, org.UserName, apiOrg.Name)
		assert.Equal(t, org.FullName, apiOrg.FullName)
		assert.Equal(t, org.Description, apiOrg.Description)
		assert.Equal(t, org.Website, apiOrg.Website)
		assert.Equal(t, org.Location, apiOrg.Location)
		assert.Equal(t, org.Visibility, apiOrg.Visibility)

		unittest.AssertExistsAndLoadBean(t, &user_model.User{
			Name:      org.UserName,
			LowerName: strings.ToLower(org.UserName),
			FullName:  org.FullName,
		})

		// Check owner team permission
		ownerTeam, _ := org_model.GetOwnerTeam(db.DefaultContext, apiOrg.ID)

		for _, ut := range unit_model.AllRepoUnitTypes {
			up := perm.AccessModeOwner
			if ut == unit_model.TypeExternalTracker || ut == unit_model.TypeExternalWiki {
				up = perm.AccessModeRead
			}
			unittest.AssertExistsAndLoadBean(t, &org_model.TeamUnit{
				OrgID:      apiOrg.ID,
				TeamID:     ownerTeam.ID,
				Type:       ut,
				AccessMode: up,
			})
		}

		req = NewRequestf(t, "GET", "/api/v1/orgs/%s?token=%s", org.UserName, token)
		resp = MakeRequest(t, req, http.StatusOK)
		DecodeJSON(t, resp, &apiOrg)
		assert.EqualValues(t, org.UserName, apiOrg.Name)

		req = NewRequestf(t, "GET", "/api/v1/orgs/%s/repos?token=%s", org.UserName, token)
		resp = MakeRequest(t, req, http.StatusOK)

		var repos []*api.Repository
		DecodeJSON(t, resp, &repos)
		for _, repo := range repos {
			assert.False(t, repo.Private)
		}

		req = NewRequestf(t, "GET", "/api/v1/orgs/%s/members?token=%s", org.UserName, token)
		resp = MakeRequest(t, req, http.StatusOK)

		// user1 on this org is public
		var users []*api.User
		DecodeJSON(t, resp, &users)
		assert.Len(t, users, 1)
		assert.EqualValues(t, "user1", users[0].UserName)
	})
}

func TestAPIOrgEdit(t *testing.T) {
	onGiteaRun(t, func(*testing.T, *url.URL) {
		session := loginUser(t, "user1")

		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteOrganization)
		org := api.EditOrgOption{
			FullName:    "User3 organization new full name",
			Description: "A new description",
			Website:     "https://try.gitea.io/new",
			Location:    "Beijing",
			Visibility:  "private",
		}
		req := NewRequestWithJSON(t, "PATCH", "/api/v1/orgs/user3?token="+token, &org)
		resp := MakeRequest(t, req, http.StatusOK)

		var apiOrg api.Organization
		DecodeJSON(t, resp, &apiOrg)

		assert.Equal(t, "user3", apiOrg.Name)
		assert.Equal(t, org.FullName, apiOrg.FullName)
		assert.Equal(t, org.Description, apiOrg.Description)
		assert.Equal(t, org.Website, apiOrg.Website)
		assert.Equal(t, org.Location, apiOrg.Location)
		assert.Equal(t, org.Visibility, apiOrg.Visibility)
	})
}

func TestAPIOrgEditBadVisibility(t *testing.T) {
	onGiteaRun(t, func(*testing.T, *url.URL) {
		session := loginUser(t, "user1")

		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteOrganization)
		org := api.EditOrgOption{
			FullName:    "User3 organization new full name",
			Description: "A new description",
			Website:     "https://try.gitea.io/new",
			Location:    "Beijing",
			Visibility:  "badvisibility",
		}
		req := NewRequestWithJSON(t, "PATCH", "/api/v1/orgs/user3?token="+token, &org)
		MakeRequest(t, req, http.StatusUnprocessableEntity)
	})
}

func TestAPIOrgDeny(t *testing.T) {
	onGiteaRun(t, func(*testing.T, *url.URL) {
		setting.Service.RequireSignInView = true
		defer func() {
			setting.Service.RequireSignInView = false
		}()

		orgName := "user1_org"
		req := NewRequestf(t, "GET", "/api/v1/orgs/%s", orgName)
		MakeRequest(t, req, http.StatusNotFound)

		req = NewRequestf(t, "GET", "/api/v1/orgs/%s/repos", orgName)
		MakeRequest(t, req, http.StatusNotFound)

		req = NewRequestf(t, "GET", "/api/v1/orgs/%s/members", orgName)
		MakeRequest(t, req, http.StatusNotFound)
	})
}

func TestAPIGetAll(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	token := getUserToken(t, "user1", auth_model.AccessTokenScopeReadOrganization)

	// accessing with a token will return all orgs
	req := NewRequestf(t, "GET", "/api/v1/orgs?token=%s", token)
	resp := MakeRequest(t, req, http.StatusOK)
	var apiOrgList []*api.Organization

	DecodeJSON(t, resp, &apiOrgList)
	assert.Len(t, apiOrgList, 9)
	assert.Equal(t, "org25", apiOrgList[1].FullName)
	assert.Equal(t, "public", apiOrgList[1].Visibility)

	// accessing without a token will return only public orgs
	req = NewRequestf(t, "GET", "/api/v1/orgs")
	resp = MakeRequest(t, req, http.StatusOK)

	DecodeJSON(t, resp, &apiOrgList)
	assert.Len(t, apiOrgList, 7)
	assert.Equal(t, "org25", apiOrgList[0].FullName)
	assert.Equal(t, "public", apiOrgList[0].Visibility)
}

func TestAPIOrgSearchEmptyTeam(t *testing.T) {
	onGiteaRun(t, func(*testing.T, *url.URL) {
		token := getUserToken(t, "user1", auth_model.AccessTokenScopeWriteOrganization)
		orgName := "org_with_empty_team"

		// create org
		req := NewRequestWithJSON(t, "POST", "/api/v1/orgs?token="+token, &api.CreateOrgOption{
			UserName: orgName,
		})
		MakeRequest(t, req, http.StatusCreated)

		// create team with no member
		req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/orgs/%s/teams?token=%s", orgName, token), &api.CreateTeamOption{
			Name:                    "Empty",
			IncludesAllRepositories: true,
			Permission:              "read",
			Units:                   []string{"repo.code", "repo.issues", "repo.ext_issues", "repo.wiki", "repo.pulls"},
		})
		MakeRequest(t, req, http.StatusCreated)

		// case-insensitive search for teams that have no members
		req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/orgs/%s/teams/search?q=%s&token=%s", orgName, "empty", token))
		resp := MakeRequest(t, req, http.StatusOK)
		data := struct {
			Ok   bool
			Data []*api.Team
		}{}
		DecodeJSON(t, resp, &data)
		assert.True(t, data.Ok)
		if assert.Len(t, data.Data, 1) {
			assert.EqualValues(t, "Empty", data.Data[0].Name)
		}
	})
}
