// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"sort"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	"gitea.dev/models/organization"
	"gitea.dev/models/perm"
	"gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/structs"
	api "gitea.dev/modules/structs"
	"gitea.dev/services/convert"
	"gitea.dev/tests"

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

	apiTeam := DecodeJSON(t, resp, &api.Team{})
	assert.Equal(t, team.ID, apiTeam.ID)
	assert.Equal(t, team.Name, apiTeam.Name)
	assert.Equal(t, convert.ToOrganization(t.Context(), org), apiTeam.Organization)

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
	apiTeam = DecodeJSON(t, resp, &api.Team{})
	checkTeamResponse(t, "CreateTeam1", apiTeam, teamToCreate.Name, teamToCreate.Description, teamToCreate.IncludesAllRepositories,
		api.AccessLevelNameNone, teamToCreate.Units, nil)
	checkTeamBean(t, apiTeam.ID, teamToCreate.Name, teamToCreate.Description, teamToCreate.IncludesAllRepositories,
		api.AccessLevelNameNone, teamToCreate.Units, nil)
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
	apiTeam = DecodeJSON(t, resp, &api.Team{})
	checkTeamResponse(t, "EditTeam1", apiTeam, teamToEdit.Name, *teamToEdit.Description, *teamToEdit.IncludesAllRepositories,
		api.AccessLevelName(teamToEdit.Permission), unit.AllUnitKeyNames(), nil)
	checkTeamBean(t, apiTeam.ID, teamToEdit.Name, *teamToEdit.Description, *teamToEdit.IncludesAllRepositories,
		api.AccessLevelName(teamToEdit.Permission), unit.AllUnitKeyNames(), nil)

	// Edit team Description only
	editDescription = "first team"
	teamToEditDesc := api.EditTeamOption{Description: &editDescription}
	req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/teams/%d", teamID), teamToEditDesc).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiTeam = DecodeJSON(t, resp, &api.Team{})
	checkTeamResponse(t, "EditTeam1_DescOnly", apiTeam, teamToEdit.Name, *teamToEditDesc.Description, *teamToEdit.IncludesAllRepositories,
		api.AccessLevelName(teamToEdit.Permission), unit.AllUnitKeyNames(), nil)
	checkTeamBean(t, apiTeam.ID, teamToEdit.Name, *teamToEditDesc.Description, *teamToEdit.IncludesAllRepositories,
		api.AccessLevelName(teamToEdit.Permission), unit.AllUnitKeyNames(), nil)

	// Read team.
	teamRead := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: teamID})
	assert.NoError(t, teamRead.LoadUnits(t.Context()))
	req = NewRequestf(t, "GET", "/api/v1/teams/%d", teamID).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiTeam = DecodeJSON(t, resp, &api.Team{})
	checkTeamResponse(t, "ReadTeam1", apiTeam, teamRead.Name, *teamToEditDesc.Description, teamRead.IncludesAllRepositories,
		api.AccessLevelName(teamRead.AccessMode.ToString()), teamRead.GetUnitNames(), teamRead.GetUnitsMap())

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
	apiTeam = DecodeJSON(t, resp, &api.Team{})
	checkTeamResponse(t, "CreateTeam2", apiTeam, teamToCreate.Name, teamToCreate.Description, teamToCreate.IncludesAllRepositories,
		api.AccessLevelNameNone, nil, teamToCreate.UnitsMap)
	checkTeamBean(t, apiTeam.ID, teamToCreate.Name, teamToCreate.Description, teamToCreate.IncludesAllRepositories,
		api.AccessLevelNameNone, nil, teamToCreate.UnitsMap)
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
	apiTeam = DecodeJSON(t, resp, &api.Team{})
	checkTeamResponse(t, "EditTeam2", apiTeam, teamToEdit.Name, *teamToEdit.Description, *teamToEdit.IncludesAllRepositories,
		api.AccessLevelNameNone, nil, teamToEdit.UnitsMap)
	checkTeamBean(t, apiTeam.ID, teamToEdit.Name, *teamToEdit.Description, *teamToEdit.IncludesAllRepositories,
		api.AccessLevelNameNone, nil, teamToEdit.UnitsMap)

	// Edit team Description only
	editDescription = "second team"
	teamToEditDesc = api.EditTeamOption{Description: &editDescription}
	req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/teams/%d", teamID), teamToEditDesc).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiTeam = DecodeJSON(t, resp, &api.Team{})
	checkTeamResponse(t, "EditTeam2_DescOnly", apiTeam, teamToEdit.Name, *teamToEditDesc.Description, *teamToEdit.IncludesAllRepositories,
		api.AccessLevelNameNone, nil, teamToEdit.UnitsMap)
	checkTeamBean(t, apiTeam.ID, teamToEdit.Name, *teamToEditDesc.Description, *teamToEdit.IncludesAllRepositories,
		api.AccessLevelNameNone, nil, teamToEdit.UnitsMap)

	// Read team.
	teamRead = unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: teamID})
	req = NewRequestf(t, "GET", "/api/v1/teams/%d", teamID).
		AddTokenAuth(token)
	resp = MakeRequest(t, req, http.StatusOK)
	apiTeam = DecodeJSON(t, resp, &api.Team{})
	assert.NoError(t, teamRead.LoadUnits(t.Context()))
	checkTeamResponse(t, "ReadTeam2", apiTeam, teamRead.Name, *teamToEditDesc.Description, teamRead.IncludesAllRepositories,
		api.AccessLevelName(teamRead.AccessMode.ToString()), teamRead.GetUnitNames(), teamRead.GetUnitsMap())

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
	apiTeam = DecodeJSON(t, resp, &api.Team{})
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

func checkTeamResponse(t *testing.T, testName string, apiTeam *api.Team, name, description string, includesAllRepositories bool, permission api.AccessLevelName, units []string, unitsMap map[string]string) {
	t.Run(testName, func(t *testing.T) {
		assert.Equal(t, name, apiTeam.Name, "name")
		assert.Equal(t, description, apiTeam.Description, "description")
		assert.Equal(t, includesAllRepositories, apiTeam.IncludesAllRepositories, "includesAllRepositories")
		assert.Equal(t, permission, apiTeam.Permission, "permission")
		if units != nil {
			sort.StringSlice(units).Sort()
			sort.StringSlice(apiTeam.Units).Sort()
			assert.Equal(t, units, apiTeam.Units, "units")
		}
		if unitsMap != nil {
			assert.Equal(t, unitsMap, apiTeam.UnitsMap, "unitsMap")
		}
	})
}

func checkTeamBean(t *testing.T, id int64, name, description string, includesAllRepositories bool, permission api.AccessLevelName, units []string, unitsMap map[string]string) {
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: id})
	assert.NoError(t, team.LoadUnits(t.Context()), "LoadUnits")
	apiTeam, err := convert.ToTeam(t.Context(), team)
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

	token := getUserToken(t, user.Name, auth_model.AccessTokenScopeReadOrganization)
	req := NewRequestf(t, "GET", "/api/v1/orgs/%s/teams/search?q=%s", org.Name, "_team").
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	results := DecodeJSON(t, resp, &TeamSearchResults{})
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

	token := getUserToken(t, user.Name, auth_model.AccessTokenScopeReadOrganization)
	req := NewRequestf(t, "GET", "/api/v1/teams/%d/repos/%s/", team.ID, teamRepo.FullName()).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &api.Repository{})
	assert.Equal(t, "big_test_private_4", teamRepo.Name)

	// no access if not organization member
	user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	token5 := getUserToken(t, user5.Name, auth_model.AccessTokenScopeReadOrganization)

	req = NewRequestf(t, "GET", "/api/v1/teams/%d/repos/%s/", team.ID, teamRepo.FullName()).
		AddTokenAuth(token5)
	MakeRequest(t, req, http.StatusNotFound)
}

func TestAPITeamVisibilityAccess(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	insertTestTeam := func(t *testing.T, orgID int64, name string, visibility structs.VisibleType) *organization.Team {
		t.Helper()
		team := &organization.Team{
			OrgID:      orgID,
			LowerName:  name,
			Name:       name,
			AccessMode: perm.AccessModeRead,
			Visibility: visibility,
		}
		assert.NoError(t, db.Insert(t.Context(), team))
		return team
	}

	limitedTeam := insertTestTeam(t, 3, "limited-team", structs.VisibleTypeLimited)

	// Org member who can read a limited team must not mutate its repos without membership.
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	token := getUserToken(t, user4.Name, auth_model.AccessTokenScopeWriteOrganization)
	req := NewRequestf(t, "PUT", "/api/v1/teams/%d/repos/org3/repo3", limitedTeam.ID).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusForbidden)

	publicTeam := insertTestTeam(t, 23, "public-team", structs.VisibleTypePublic)

	// Public team in a private org must not be readable by outsiders.
	outsider := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	token = getUserToken(t, outsider.Name, auth_model.AccessTokenScopeReadOrganization)
	req = NewRequestf(t, "GET", "/api/v1/teams/%d", publicTeam.ID).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)

	// Member lookup must require org membership even for public teams.
	req = NewRequestf(t, "GET", "/api/v1/teams/%d/members/%s", publicTeam.ID, outsider.Name).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)

	// A limited team's repo list must not leak repos the viewer cannot access.
	// repo3 is private; user28 is an org3 member (team12, no repo access) who can
	// read the limited team but has no access to repo3.
	assert.NoError(t, db.Insert(t.Context(), &organization.TeamRepo{OrgID: 3, TeamID: limitedTeam.ID, RepoID: 3}))
	user28 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 28})
	token28 := getUserToken(t, user28.Name, auth_model.AccessTokenScopeReadOrganization)

	req = NewRequestf(t, "GET", "/api/v1/teams/%d/repos", limitedTeam.ID).AddTokenAuth(token28)
	resp := MakeRequest(t, req, http.StatusOK)
	var repos []*api.Repository
	DecodeJSON(t, resp, &repos)
	for _, r := range repos {
		assert.NotEqual(t, int64(3), r.ID, "must not leak inaccessible private repo3")
	}

	// The single-repo lookup must not confirm an inaccessible repo's existence.
	req = NewRequestf(t, "GET", "/api/v1/teams/%d/repos/org3/repo3", limitedTeam.ID).AddTokenAuth(token28)
	MakeRequest(t, req, http.StatusNotFound)
}
