// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/organization"
	"gitea.dev/models/perm"
	"gitea.dev/models/unit"
	api "gitea.dev/modules/structs"
	shared_user "gitea.dev/routers/web/shared/user"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIssueUnitAccess(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// user4 reaches the private org3/repo3 (issue6 by user1, pull6 by user2) only through team 2
	_, err := db.Exec(t.Context(), "UPDATE team SET authorize = ? WHERE id = ?", perm.AccessModeNone, 2)
	require.NoError(t, err)
	setTeamUnits := func(t *testing.T, units ...unit.Type) {
		_, err := db.DeleteByBean(t.Context(), &organization.TeamUnit{TeamID: 2})
		require.NoError(t, err)
		for _, u := range units {
			require.NoError(t, db.Insert(t.Context(), &organization.TeamUnit{OrgID: 3, TeamID: 2, Type: u, AccessMode: perm.AccessModeRead}))
		}
	}

	milestone := &issues_model.Milestone{RepoID: 3, Name: "unit-access-milestone"}
	require.NoError(t, issues_model.NewMilestone(t.Context(), milestone))

	session := loginUser(t, "user4")
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadIssue)
	searchTitles := func(t *testing.T, query string) (titles []string) {
		resp := MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/issues/search?state=all"+query).AddTokenAuth(token), http.StatusOK)
		for _, issue := range *DecodeJSON(t, resp, &[]*api.Issue{}) {
			titles = append(titles, issue.Title)
		}
		return titles
	}
	pageBody := func(t *testing.T, url string) string {
		return session.MakeRequest(t, NewRequest(t, "GET", url), http.StatusOK).Body.String()
	}

	t.Run("CodeOnly", func(t *testing.T) {
		setTeamUnits(t, unit.TypeCode)
		assert.NotContains(t, searchTitles(t, ""), "issue6")
		assert.NotContains(t, searchTitles(t, "&owner=org3&type=pulls"), "pull6")
		assert.NotContains(t, pageBody(t, "/org/org3/issues"), "issue6")
		assert.NotContains(t, pageBody(t, "/milestones"), milestone.Name)
	})

	t.Run("IssuesOnly", func(t *testing.T) {
		setTeamUnits(t, unit.TypeIssues)
		assert.Contains(t, searchTitles(t, "&type=issues"), "issue6")
		assert.NotContains(t, searchTitles(t, ""), "pull6") // an untyped search needs both units
		assert.Contains(t, pageBody(t, "/org/org3/issues"), "issue6")
	})

	t.Run("PullsOnly", func(t *testing.T) {
		setTeamUnits(t, unit.TypePullRequests)
		var names []string
		for _, info := range DecodeJSON(t, session.MakeRequest(t, NewRequest(t, "GET", "/org3/repo3/issues/posters"), http.StatusOK), &shared_user.SearchUserResponse{}).Results {
			names = append(names, info.UserName)
		}
		assert.Equal(t, []string{"user4", "user2"}, names, "the doer, then only the pull request poster")
	})
}
