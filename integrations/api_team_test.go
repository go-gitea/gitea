// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"sort"
	"testing"

	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/convert"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestAPITeam(t *testing.T) {
	defer prepareTestEnv(t)()

	teamUser := unittest.AssertExistsAndLoadBean(t, &organization.TeamUser{}).(*organization.TeamUser)
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: teamUser.TeamID}).(*organization.Team)
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: teamUser.UID}).(*user_model.User)

	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session)
	req := NewRequestf(t, "GET", "/api/v1/teams/%d?token="+token, teamUser.TeamID)
	resp := session.MakeRequest(t, req, http.StatusOK)

	var apiTeam api.Team
	DecodeJSON(t, resp, &apiTeam)
	assert.EqualValues(t, team.ID, apiTeam.ID)
	assert.Equal(t, team.Name, apiTeam.Name)

	// non team member user will not access the teams details
	teamUser2 := unittest.AssertExistsAndLoadBean(t, &organization.TeamUser{ID: 3}).(*organization.TeamUser)
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: teamUser2.UID}).(*user_model.User)

	session = loginUser(t, user2.Name)
	token = getTokenForLoggedInUser(t, session)
	req = NewRequestf(t, "GET", "/api/v1/teams/%d?token="+token, teamUser.TeamID)
	_ = session.MakeRequest(t, req, http.StatusForbidden)

	req = NewRequestf(t, "GET", "/api/v1/teams/%d", teamUser.TeamID)
	_ = session.MakeRequest(t, req, http.StatusUnauthorized)

	// Get an admin user able to create, update and delete teams.
	user = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1}).(*user_model.User)
	session = loginUser(t, user.Name)
	token = getTokenForLoggedInUser(t, session)

	org := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 6}).(*user_model.User)

	// Create team.
	teamToCreate := &api.CreateTeamOption{
		Name:                    "team1",
		Description:             "team one",
		IncludesAllRepositories: true,
		Permission:              "write",
		Units:                   []string{"repo.code", "repo.issues"},
	}
	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/orgs/%s/teams?token=%s", org.Name, token), teamToCreate)
	resp = session.MakeRequest(t, req, http.StatusCreated)
	apiTeam = api.Team{}
	DecodeJSON(t, resp, &apiTeam)
	checkTeamResponse(t, &apiTeam, teamToCreate.Name, teamToCreate.Description, teamToCreate.IncludesAllRepositories,
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

	req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/teams/%d?token=%s", teamID, token), teamToEdit)
	resp = session.MakeRequest(t, req, http.StatusOK)
	apiTeam = api.Team{}
	DecodeJSON(t, resp, &apiTeam)
	checkTeamResponse(t, &apiTeam, teamToEdit.Name, *teamToEdit.Description, *teamToEdit.IncludesAllRepositories,
		teamToEdit.Permission, unit.AllUnitKeyNames(), nil)
	checkTeamBean(t, apiTeam.ID, teamToEdit.Name, *teamToEdit.Description, *teamToEdit.IncludesAllRepositories,
		teamToEdit.Permission, unit.AllUnitKeyNames(), nil)

	// Edit team Description only
	editDescription = "first team"
	teamToEditDesc := api.EditTeamOption{Description: &editDescription}
	req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/teams/%d?token=%s", teamID, token), teamToEditDesc)
	resp = session.MakeRequest(t, req, http.StatusOK)
	apiTeam = api.Team{}
	DecodeJSON(t, resp, &apiTeam)
	checkTeamResponse(t, &apiTeam, teamToEdit.Name, *teamToEditDesc.Description, *teamToEdit.IncludesAllRepositories,
		teamToEdit.Permission, unit.AllUnitKeyNames(), nil)
	checkTeamBean(t, apiTeam.ID, teamToEdit.Name, *teamToEditDesc.Description, *teamToEdit.IncludesAllRepositories,
		teamToEdit.Permission, unit.AllUnitKeyNames(), nil)

	// Read team.
	teamRead := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: teamID}).(*organization.Team)
	assert.NoError(t, teamRead.GetUnits())
	req = NewRequestf(t, "GET", "/api/v1/teams/%d?token="+token, teamID)
	resp = session.MakeRequest(t, req, http.StatusOK)
	apiTeam = api.Team{}
	DecodeJSON(t, resp, &apiTeam)
	checkTeamResponse(t, &apiTeam, teamRead.Name, *teamToEditDesc.Description, teamRead.IncludesAllRepositories,
		teamRead.AccessMode.String(), teamRead.GetUnitNames(), teamRead.GetUnitsMap())

	// Delete team.
	req = NewRequestf(t, "DELETE", "/api/v1/teams/%d?token="+token, teamID)
	session.MakeRequest(t, req, http.StatusNoContent)
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
	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/orgs/%s/teams?token=%s", org.Name, token), teamToCreate)
	resp = session.MakeRequest(t, req, http.StatusCreated)
	apiTeam = api.Team{}
	DecodeJSON(t, resp, &apiTeam)
	checkTeamResponse(t, &apiTeam, teamToCreate.Name, teamToCreate.Description, teamToCreate.IncludesAllRepositories,
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

	req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/teams/%d?token=%s", teamID, token), teamToEdit)
	resp = session.MakeRequest(t, req, http.StatusOK)
	apiTeam = api.Team{}
	DecodeJSON(t, resp, &apiTeam)
	checkTeamResponse(t, &apiTeam, teamToEdit.Name, *teamToEdit.Description, *teamToEdit.IncludesAllRepositories,
		"read", nil, teamToEdit.UnitsMap)
	checkTeamBean(t, apiTeam.ID, teamToEdit.Name, *teamToEdit.Description, *teamToEdit.IncludesAllRepositories,
		"read", nil, teamToEdit.UnitsMap)

	// Edit team Description only
	editDescription = "second team"
	teamToEditDesc = api.EditTeamOption{Description: &editDescription}
	req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/teams/%d?token=%s", teamID, token), teamToEditDesc)
	resp = session.MakeRequest(t, req, http.StatusOK)
	apiTeam = api.Team{}
	DecodeJSON(t, resp, &apiTeam)
	checkTeamResponse(t, &apiTeam, teamToEdit.Name, *teamToEditDesc.Description, *teamToEdit.IncludesAllRepositories,
		"read", nil, teamToEdit.UnitsMap)
	checkTeamBean(t, apiTeam.ID, teamToEdit.Name, *teamToEditDesc.Description, *teamToEdit.IncludesAllRepositories,
		"read", nil, teamToEdit.UnitsMap)

	// Read team.
	teamRead = unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: teamID}).(*organization.Team)
	req = NewRequestf(t, "GET", "/api/v1/teams/%d?token="+token, teamID)
	resp = session.MakeRequest(t, req, http.StatusOK)
	apiTeam = api.Team{}
	DecodeJSON(t, resp, &apiTeam)
	assert.NoError(t, teamRead.GetUnits())
	checkTeamResponse(t, &apiTeam, teamRead.Name, *teamToEditDesc.Description, teamRead.IncludesAllRepositories,
		teamRead.AccessMode.String(), teamRead.GetUnitNames(), teamRead.GetUnitsMap())

	// Delete team.
	req = NewRequestf(t, "DELETE", "/api/v1/teams/%d?token="+token, teamID)
	session.MakeRequest(t, req, http.StatusNoContent)
	unittest.AssertNotExistsBean(t, &organization.Team{ID: teamID})
}

func checkTeamResponse(t *testing.T, apiTeam *api.Team, name, description string, includesAllRepositories bool, permission string, units []string, unitsMap map[string]string) {
	t.Run(name+description, func(t *testing.T) {
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
	team := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: id}).(*organization.Team)
	assert.NoError(t, team.GetUnits(), "GetUnits")
	checkTeamResponse(t, convert.ToTeam(team), name, description, includesAllRepositories, permission, units, unitsMap)
}

type TeamSearchResults struct {
	OK   bool        `json:"ok"`
	Data []*api.Team `json:"data"`
}

func TestAPITeamSearch(t *testing.T) {
	defer prepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	org := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3}).(*user_model.User)

	var results TeamSearchResults

	token := getUserToken(t, user.Name)
	req := NewRequestf(t, "GET", "/api/v1/orgs/%s/teams/search?q=%s&token=%s", org.Name, "_team", token)
	resp := MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &results)
	assert.NotEmpty(t, results.Data)
	assert.Len(t, results.Data, 1)
	assert.Equal(t, "test_team", results.Data[0].Name)

	// no access if not organization member
	user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5}).(*user_model.User)
	token5 := getUserToken(t, user5.Name)

	req = NewRequestf(t, "GET", "/api/v1/orgs/%s/teams/search?q=%s&token=%s", org.Name, "team", token5)
	MakeRequest(t, req, http.StatusForbidden)
}
