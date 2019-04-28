// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"sort"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/routers/api/v1/convert"
	api "code.gitea.io/sdk/gitea"

	"github.com/stretchr/testify/assert"
)

func TestAPITeam(t *testing.T) {
	prepareTestEnv(t)

	teamUser := models.AssertExistsAndLoadBean(t, &models.TeamUser{}).(*models.TeamUser)
	team := models.AssertExistsAndLoadBean(t, &models.Team{ID: teamUser.TeamID}).(*models.Team)
	user := models.AssertExistsAndLoadBean(t, &models.User{ID: teamUser.UID}).(*models.User)

	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session)
	req := NewRequestf(t, "GET", "/api/v1/teams/%d?token="+token, teamUser.TeamID)
	resp := session.MakeRequest(t, req, http.StatusOK)

	var apiTeam api.Team
	DecodeJSON(t, resp, &apiTeam)
	assert.EqualValues(t, team.ID, apiTeam.ID)
	assert.Equal(t, team.Name, apiTeam.Name)

	// non team member user will not access the teams details
	teamUser2 := models.AssertExistsAndLoadBean(t, &models.TeamUser{ID: 3}).(*models.TeamUser)
	user2 := models.AssertExistsAndLoadBean(t, &models.User{ID: teamUser2.UID}).(*models.User)

	session = loginUser(t, user2.Name)
	token = getTokenForLoggedInUser(t, session)
	req = NewRequestf(t, "GET", "/api/v1/teams/%d?token="+token, teamUser.TeamID)
	resp = session.MakeRequest(t, req, http.StatusForbidden)

	req = NewRequestf(t, "GET", "/api/v1/teams/%d", teamUser.TeamID)
	resp = session.MakeRequest(t, req, http.StatusUnauthorized)

	// Get a member of the owner team able to create, update and delete teams.
	user = models.AssertExistsAndLoadBean(t, &models.User{ID: 5}).(*models.User)
	session = loginUser(t, user.Name)
	token = getTokenForLoggedInUser(t, session)

	org := models.AssertExistsAndLoadBean(t, &models.User{ID: 6}).(*models.User)

	// Create team.
	teamToCreate := &api.CreateTeamOption{
		Name:        "team1",
		Description: "team one",
		Permission:  "write",
		Units:       []string{"repo.code", "repo.issues"},
	}
	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/orgs/%s/teams?token=%s", org.Name, token), teamToCreate)
	resp = session.MakeRequest(t, req, http.StatusCreated)
	DecodeJSON(t, resp, &apiTeam)
	checkTeamResponse(t, &apiTeam, teamToCreate.Name, teamToCreate.Description, teamToCreate.Permission, teamToCreate.Units)
	checkTeamBean(t, apiTeam.ID, teamToCreate.Name, teamToCreate.Description, teamToCreate.Permission, teamToCreate.Units)
	teamID := apiTeam.ID

	// Edit team.
	teamToEdit := &api.EditTeamOption{
		Name:        "teamone",
		Description: "team 1",
		Permission:  "admin",
		Units:       []string{"repo.code", "repo.pulls", "repo.releases"},
	}
	req = NewRequestWithJSON(t, "PATCH", fmt.Sprintf("/api/v1/teams/%d?token=%s", teamID, token), teamToEdit)
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiTeam)
	checkTeamResponse(t, &apiTeam, teamToEdit.Name, teamToEdit.Description, teamToEdit.Permission, teamToEdit.Units)
	checkTeamBean(t, apiTeam.ID, teamToEdit.Name, teamToEdit.Description, teamToEdit.Permission, teamToEdit.Units)

	// Read team.
	teamRead := models.AssertExistsAndLoadBean(t, &models.Team{ID: teamID}).(*models.Team)
	req = NewRequestf(t, "GET", "/api/v1/teams/%d?token="+token, teamID)
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiTeam)
	checkTeamResponse(t, &apiTeam, teamRead.Name, teamRead.Description, teamRead.Authorize.String(), teamRead.GetUnitNames())

	// Delete team.
	req = NewRequestf(t, "DELETE", "/api/v1/teams/%d?token="+token, teamID)
	session.MakeRequest(t, req, http.StatusNoContent)
	models.AssertNotExistsBean(t, &models.Team{ID: teamID})
}

func checkTeamResponse(t *testing.T, apiTeam *api.Team, name, description string, permission string, units []string) {
	assert.Equal(t, name, apiTeam.Name, "name")
	assert.Equal(t, description, apiTeam.Description, "description")
	assert.Equal(t, permission, apiTeam.Permission, "permission")
	sort.StringSlice(units).Sort()
	sort.StringSlice(apiTeam.Units).Sort()
	assert.EqualValues(t, units, apiTeam.Units, "units")
}

func checkTeamBean(t *testing.T, id int64, name, description string, permission string, units []string) {
	team := models.AssertExistsAndLoadBean(t, &models.Team{ID: id}).(*models.Team)
	assert.NoError(t, team.GetUnits(), "GetUnits")
	checkTeamResponse(t, convert.ToTeam(team), name, description, permission, units)
}
