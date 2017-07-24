// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/sdk/gitea"

	"github.com/stretchr/testify/assert"
)

func TestAPITeam(t *testing.T) {
	prepareTestEnv(t)
	teamUser := models.AssertExistsAndLoadBean(t, &models.TeamUser{}).(*models.TeamUser)
	team := models.AssertExistsAndLoadBean(t, &models.Team{ID: teamUser.TeamID}).(*models.Team)
	user := models.AssertExistsAndLoadBean(t, &models.User{ID: teamUser.UID}).(*models.User)

	session := loginUser(t, user.Name)
	req := NewRequestf(t, "GET", "/api/v1/teams/%d", teamUser.TeamID)
	resp := session.MakeRequest(t, req, http.StatusOK)

	var apiTeam api.Team
	DecodeJSON(t, resp, &apiTeam)
	assert.EqualValues(t, team.ID, apiTeam.ID)
	assert.Equal(t, team.Name, apiTeam.Name)
}
