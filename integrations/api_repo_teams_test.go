// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/unittest"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestAPIRepoTeams(t *testing.T) {
	defer prepareTestEnv(t)()

	// publicOrgRepo = user3/repo21
	publicOrgRepo := unittest.AssertExistsAndLoadBean(t, &models.Repository{ID: 32}).(*models.Repository)
	// user4
	user := unittest.AssertExistsAndLoadBean(t, &models.User{ID: 4}).(*models.User)
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session)

	// ListTeams
	url := fmt.Sprintf("/api/v1/repos/%s/teams?token=%s", publicOrgRepo.FullName(), token)
	req := NewRequest(t, "GET", url)
	res := session.MakeRequest(t, req, http.StatusOK)
	var teams []*api.Team
	DecodeJSON(t, res, &teams)
	if assert.Len(t, teams, 2) {
		assert.EqualValues(t, "Owners", teams[0].Name)
		assert.False(t, teams[0].CanCreateOrgRepo)
		assert.EqualValues(t, []string{"repo.code", "repo.issues", "repo.pulls", "repo.releases", "repo.wiki", "repo.ext_wiki", "repo.ext_issues"}, teams[0].Units)
		assert.EqualValues(t, "owner", teams[0].Permission)

		assert.EqualValues(t, "test_team", teams[1].Name)
		assert.False(t, teams[1].CanCreateOrgRepo)
		assert.EqualValues(t, []string{"repo.issues"}, teams[1].Units)
		assert.EqualValues(t, "write", teams[1].Permission)
	}

	// IsTeam
	url = fmt.Sprintf("/api/v1/repos/%s/teams/%s?token=%s", publicOrgRepo.FullName(), "Test_Team", token)
	req = NewRequest(t, "GET", url)
	res = session.MakeRequest(t, req, http.StatusOK)
	var team *api.Team
	DecodeJSON(t, res, &team)
	assert.EqualValues(t, teams[1], team)

	url = fmt.Sprintf("/api/v1/repos/%s/teams/%s?token=%s", publicOrgRepo.FullName(), "NonExistingTeam", token)
	req = NewRequest(t, "GET", url)
	res = session.MakeRequest(t, req, http.StatusNotFound)

	// AddTeam with user4
	url = fmt.Sprintf("/api/v1/repos/%s/teams/%s?token=%s", publicOrgRepo.FullName(), "team1", token)
	req = NewRequest(t, "PUT", url)
	res = session.MakeRequest(t, req, http.StatusForbidden)

	// AddTeam with user2
	user = unittest.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	session = loginUser(t, user.Name)
	token = getTokenForLoggedInUser(t, session)
	url = fmt.Sprintf("/api/v1/repos/%s/teams/%s?token=%s", publicOrgRepo.FullName(), "team1", token)
	req = NewRequest(t, "PUT", url)
	res = session.MakeRequest(t, req, http.StatusNoContent)
	res = session.MakeRequest(t, req, http.StatusUnprocessableEntity) // test duplicate request

	// DeleteTeam
	url = fmt.Sprintf("/api/v1/repos/%s/teams/%s?token=%s", publicOrgRepo.FullName(), "team1", token)
	req = NewRequest(t, "DELETE", url)
	res = session.MakeRequest(t, req, http.StatusNoContent)
	res = session.MakeRequest(t, req, http.StatusUnprocessableEntity) // test duplicate request
}
