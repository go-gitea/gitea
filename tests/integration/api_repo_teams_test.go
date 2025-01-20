// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIRepoTeams(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// publicOrgRepo = org3/repo21
	publicOrgRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 32})
	// user4
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	// ListTeams
	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/teams", publicOrgRepo.FullName())).
		AddTokenAuth(token)
	res := MakeRequest(t, req, http.StatusOK)
	var teams []*api.Team
	DecodeJSON(t, res, &teams)
	if assert.Len(t, teams, 2) {
		assert.EqualValues(t, "Owners", teams[0].Name)
		assert.True(t, teams[0].CanCreateOrgRepo)
		assert.True(t, util.SliceSortedEqual(unit.AllUnitKeyNames(), teams[0].Units), "%v == %v", unit.AllUnitKeyNames(), teams[0].Units)
		assert.EqualValues(t, "owner", teams[0].Permission)

		assert.EqualValues(t, "test_team", teams[1].Name)
		assert.False(t, teams[1].CanCreateOrgRepo)
		assert.EqualValues(t, []string{"repo.issues"}, teams[1].Units)
		assert.EqualValues(t, "write", teams[1].Permission)
	}

	// IsTeam
	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/teams/%s", publicOrgRepo.FullName(), "Test_Team")).
		AddTokenAuth(token)
	res = MakeRequest(t, req, http.StatusOK)
	var team *api.Team
	DecodeJSON(t, res, &team)
	assert.EqualValues(t, teams[1], team)

	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/teams/%s", publicOrgRepo.FullName(), "NonExistingTeam")).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNotFound)

	// AddTeam with user4
	req = NewRequest(t, "PUT", fmt.Sprintf("/api/v1/repos/%s/teams/%s", publicOrgRepo.FullName(), "team1")).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusForbidden)

	// AddTeam with user2
	user = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	session = loginUser(t, user.Name)
	token = getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	req = NewRequest(t, "PUT", fmt.Sprintf("/api/v1/repos/%s/teams/%s", publicOrgRepo.FullName(), "team1")).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)
	MakeRequest(t, req, http.StatusUnprocessableEntity) // test duplicate request

	// DeleteTeam
	req = NewRequest(t, "DELETE", fmt.Sprintf("/api/v1/repos/%s/teams/%s", publicOrgRepo.FullName(), "team1")).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)
	MakeRequest(t, req, http.StatusUnprocessableEntity) // test duplicate request
}
