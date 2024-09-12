// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"sort"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/convert"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPITeam(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	teamUser := unittest.AssertExistsAndLoadBean(t, &organization.TeamUser{ID: 1})
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: teamUser.TeamID})
	org := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: teamUser.OrgID})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: teamUser.UID})

	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadOrganization)
	req := NewRequestf(t, "GET", "/api/v1/teams/%d", teamUser.TeamID).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)

	var apiTeam api.Team
	DecodeJSON(t, resp, &apiTeam)
	assert.EqualValues(t, team.ID, apiTeam.ID)
	assert.Equal(t, team.Name, apiTeam.Name)
	assert.EqualValues(t, convert.ToOrganization(db.DefaultContext, org), apiTeam.Organization)

	// non team member user will not access the teams details
	teamUser2 := unittest.AssertExistsAndLoadBean(t, &organization.TeamUser{ID: 3})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: teamUser2.UID})

	session = loginUser(t, user2.Name)
	token = getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadOrganization)
	req = NewRequestf(t, "GET", "/api/v1/teams/%d", teamUser.TeamID).
		AddTokenAuth(token)
	_ = MakeRequest(t, req, http.StatusForbidden)

	req = NewRequestf(t, "GET", "/api/v1/teams/%d", teamUser.TeamID)
	_ = MakeRequest(t, req, http.StatusUnauthorized)

	// Get an admin user able to create, update and delete teams.
	user = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	session = loginUser(t, user.Name)
	token = getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteOrganization)

	org = unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 6})

	// Create team.
	teamToCreate := &api.CreateTeamOption{
		Name:                    "team1",
		Description:             "team one",
		IncludesAllRepositories: true,
		Permission:              "write",
		Units:                   []string{"repo.code", "repo.issues"},
	}
	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/orgs/%s/teams", org.Name), teamToCreate).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusCreated)
	apiTeam = api.Team{}
	DecodeJSON(t, resp, &apiTeam)
	checkTeamResponse(t, "CreateTeam1", &apiTeam, teamToCreate.Name, teamToCreate.Description, teamToCreate.IncludesAllRepositories,
		teamToCreate.Permission, teamToCreate.Units, nil)
	checkTeamBean(t, apiTeam.ID, teamToCreate.Name, teamToCreate.Description, teamToCreate.IncludesAllRepositories,
		teamToCreate.Permission, teamToCreate.Units, nil)
	teamID := apiTeam.ID

	// Edit team.
	editDescription := "team 1"
	editFalse := false
	teamToEdit := &api.EditTeamOption{
		Name:                    "teamone",
		Description:             &editDescription,
		Permission:              "admin",
		IncludesAllRepositories: &editFalse,
		Units:                   []string{"repo.code", "repo.pulls", "repo.releases"},
	}

	req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/teams/%d", teamID), teamToEdit).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiTeam = api.Team{}
	DecodeJSON(t, resp, &apiTeam)
	checkTeamResponse(t, "EditTeam1", &apiTeam, teamToEdit.Name, *teamToEdit.Description, *teamToEdit.IncludesAllRepositories,
		teamToEdit.Permission, unit.AllUnitKeyNames(), nil)
	checkTeamBean(t, apiTeam.ID, teamToEdit.Name, *teamToEdit.Description, *teamToEdit.IncludesAllRepositories,
		teamToEdit.Permission, unit.AllUnitKeyNames(), nil)

	// Edit team Description only
	editDescription = "first team"
	teamToEditDesc := api.EditTeamOption{Description: &editDescription}
	req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/teams/%d", teamID), teamToEditDesc).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiTeam = api.Team{}
	DecodeJSON(t, resp, &apiTeam)
	checkTeamResponse(t, "EditTeam1_DescOnly", &apiTeam, teamToEdit.Name, *teamToEditDesc.Description, *teamToEdit.IncludesAllRepositories,
		teamToEdit.Permission, unit.AllUnitKeyNames(), nil)
	checkTeamBean(t, apiTeam.ID, teamToEdit.Name, *teamToEditDesc.Description, *teamToEdit.IncludesAllRepositories,
		teamToEdit.Permission, unit.AllUnitKeyNames(), nil)

	// Read team.
	teamRead := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: teamID})
	assert.NoError(t, teamRead.LoadUnits(db.DefaultContext))
	req = NewRequestf(t, "GET", "/api/v1/teams/%d", teamID).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiTeam = api.Team{}
	DecodeJSON(t, resp, &apiTeam)
	checkTeamResponse(t, "ReadTeam1", &apiTeam, teamRead.Name, *teamToEditDesc.Description, teamRead.IncludesAllRepositories,
		teamRead.AccessMode.ToString(), teamRead.GetUnitNames(), teamRead.GetUnitsMap())

	// Delete team.
	req = NewRequestf(t, "DELETE", "/api/v1/teams/%d", teamID).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)
	unittest.AssertNotExistsBean(t, &organization.Team{ID: teamID})

	// create team again via UnitsMap
	// Create team.
	teamToCreate = &api.CreateTeamOption{
		Name:                    "team2",
		Description:             "team two",
		IncludesAllRepositories: true,
		Permission:              "write",
		UnitsMap:                map[string]string{"repo.code": "read", "repo.issues": "write", "repo.wiki": "none"},
	}
	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/orgs/%s/teams", org.Name), teamToCreate).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusCreated)
	apiTeam = api.Team{}
	DecodeJSON(t, resp, &apiTeam)
	checkTeamResponse(t, "CreateTeam2", &apiTeam, teamToCreate.Name, teamToCreate.Description, teamToCreate.IncludesAllRepositories,
		"read", nil, teamToCreate.UnitsMap)
	checkTeamBean(t, apiTeam.ID, teamToCreate.Name, teamToCreate.Description, teamToCreate.IncludesAllRepositories,
		"read", nil, teamToCreate.UnitsMap)
	teamID = apiTeam.ID

	// Edit team.
	editDescription = "team 1"
	editFalse = false
	teamToEdit = &api.EditTeamOption{
		Name:                    "teamtwo",
		Description:             &editDescription,
		Permission:              "write",
		IncludesAllRepositories: &editFalse,
		UnitsMap:                map[string]string{"repo.code": "read", "repo.pulls": "read", "repo.releases": "write"},
	}

	req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/teams/%d", teamID), teamToEdit).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiTeam = api.Team{}
	DecodeJSON(t, resp, &apiTeam)
	checkTeamResponse(t, "EditTeam2", &apiTeam, teamToEdit.Name, *teamToEdit.Description, *teamToEdit.IncludesAllRepositories,
		"read", nil, teamToEdit.UnitsMap)
	checkTeamBean(t, apiTeam.ID, teamToEdit.Name, *teamToEdit.Description, *teamToEdit.IncludesAllRepositories,
		"read", nil, teamToEdit.UnitsMap)

	// Edit team Description only
	editDescription = "second team"
	teamToEditDesc = api.EditTeamOption{Description: &editDescription}
	req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/teams/%d", teamID), teamToEditDesc).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiTeam = api.Team{}
	DecodeJSON(t, resp, &apiTeam)
	checkTeamResponse(t, "EditTeam2_DescOnly", &apiTeam, teamToEdit.Name, *teamToEditDesc.Description, *teamToEdit.IncludesAllRepositories,
		"read", nil, teamToEdit.UnitsMap)
	checkTeamBean(t, apiTeam.ID, teamToEdit.Name, *teamToEditDesc.Description, *teamToEdit.IncludesAllRepositories,
		"read", nil, teamToEdit.UnitsMap)

	// Read team.
	teamRead = unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: teamID})
	req = NewRequestf(t, "GET", "/api/v1/teams/%d", teamID).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiTeam = api.Team{}
	DecodeJSON(t, resp, &apiTeam)
	assert.NoError(t, teamRead.LoadUnits(db.DefaultContext))
	checkTeamResponse(t, "ReadTeam2", &apiTeam, teamRead.Name, *teamToEditDesc.Description, teamRead.IncludesAllRepositories,
		teamRead.AccessMode.ToString(), teamRead.GetUnitNames(), teamRead.GetUnitsMap())

	// Delete team.
	req = NewRequestf(t, "DELETE", "/api/v1/teams/%d", teamID).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)
	unittest.AssertNotExistsBean(t, &organization.Team{ID: teamID})

	// Create admin team
	teamToCreate = &api.CreateTeamOption{
		Name:                    "teamadmin",
		Description:             "team admin",
		IncludesAllRepositories: true,
		Permission:              "admin",
	}
	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/orgs/%s/teams", org.Name), teamToCreate).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusCreated)
	apiTeam = api.Team{}
	DecodeJSON(t, resp, &apiTeam)
	for _, ut := range unit.AllRepoUnitTypes {
		up := perm.AccessModeAdmin
		if ut == unit.TypeExternalTracker || ut == unit.TypeExternalWiki {
			up = perm.AccessModeRead
		}
		unittest.AssertExistsAndLoadBean(t, &organization.TeamUnit{
			OrgID:      org.ID,
			TeamID:     apiTeam.ID,
			Type:       ut,
			AccessMode: up,
		})
	}
	teamID = apiTeam.ID

	// Delete team.
	req = NewRequestf(t, "DELETE", "/api/v1/teams/%d", teamID).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)
	unittest.AssertNotExistsBean(t, &organization.Team{ID: teamID})
}

func checkTeamResponse(t *testing.T, testName string, apiTeam *api.Team, name, description string, includesAllRepositories bool, permission string, units []string, unitsMap map[string]string) {
	t.Run(testName, func(t *testing.T) {
		assert.Equal(t, name, apiTeam.Name, "name")
		assert.Equal(t, description, apiTeam.Description, "description")
		assert.Equal(t, includesAllRepositories, apiTeam.IncludesAllRepositories, "includesAllRepositories")
		assert.Equal(t, permission, apiTeam.Permission, "permission")
		if units != nil {
			sort.StringSlice(units).Sort()
			sort.StringSlice(apiTeam.Units).Sort()
			assert.EqualValues(t, units, apiTeam.Units, "units")
		}
		if unitsMap != nil {
			assert.EqualValues(t, unitsMap, apiTeam.UnitsMap, "unitsMap")
		}
	})
}

func checkTeamBean(t *testing.T, id int64, name, description string, includesAllRepositories bool, permission string, units []string, unitsMap map[string]string) {
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: id})
	assert.NoError(t, team.LoadUnits(db.DefaultContext), "LoadUnits")
	apiTeam, err := convert.ToTeam(db.DefaultContext, team)
	assert.NoError(t, err)
	checkTeamResponse(t, fmt.Sprintf("checkTeamBean/%s_%s", name, description), apiTeam, name, description, includesAllRepositories, permission, units, unitsMap)
}

type TeamSearchResults struct {
	OK   bool        `json:"ok"`
	Data []*api.Team `json:"data"`
}

func TestAPITeamSearch(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	org := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 17})

	var results TeamSearchResults

	token := getUserToken(t, user.Name, auth_model.AccessTokenScopeReadOrganization)
	req := NewRequestf(t, "GET", "/api/v1/orgs/%s/teams/search?q=%s", org.Name, "_team").
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &results)
	assert.NotEmpty(t, results.Data)
	assert.Len(t, results.Data, 1)
	assert.Equal(t, "test_team", results.Data[0].Name)

	// no access if not organization member
	user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	token5 := getUserToken(t, user5.Name, auth_model.AccessTokenScopeReadOrganization)

	req = NewRequestf(t, "GET", "/api/v1/orgs/%s/teams/search?q=%s", org.Name, "team").
		AddTokenAuth(token5)
	MakeRequest(t, req, http.StatusForbidden)
}

func TestAPIGetTeamRepo(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 15})
	teamRepo := unittest.AssertExistsAndLoadBean(t, &repo.Repository{ID: 24})
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 5})

	var results api.Repository

	token := getUserToken(t, user.Name, auth_model.AccessTokenScopeReadOrganization)
	req := NewRequestf(t, "GET", "/api/v1/teams/%d/repos/%s/", team.ID, teamRepo.FullName()).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &results)
	assert.Equal(t, "big_test_private_4", teamRepo.Name)

	// no access if not organization member
	user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	token5 := getUserToken(t, user5.Name, auth_model.AccessTokenScopeReadOrganization)

	req = NewRequestf(t, "GET", "/api/v1/teams/%d/repos/%s/", team.ID, teamRepo.FullName()).
		AddTokenAuth(token5)
	MakeRequest(t, req, http.StatusNotFound)
}
