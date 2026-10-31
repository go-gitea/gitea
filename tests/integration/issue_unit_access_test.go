// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	activities_model "gitea.dev/models/activities"
	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/organization"
	"gitea.dev/models/perm"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIssueUnitAccess(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// user4 reaches the private org3/repo3 (issue6 is an issue, pull6 a pull request) only through team 2
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
	_, err = db.Exec(t.Context(), "UPDATE issue SET milestone_id = ? WHERE repo_id = 3", milestone.ID)
	require.NoError(t, err)
	require.NoError(t, db.Insert(t.Context(), &activities_model.Action{
		UserID: 2, ActUserID: 2, OpType: activities_model.ActionCreateIssue, RepoID: 3, IsPrivate: true, Content: "1|issue6",
	}))

	session := loginUser(t, "user4")
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadIssue, auth_model.AccessTokenScopeReadRepository)
	searchTitles := func(t *testing.T, query string) (titles []string) {
		resp := MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/issues/search?state=all&owner=org3"+query).AddTokenAuth(token), http.StatusOK)
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
		assert.NotContains(t, searchTitles(t, "&type=pulls"), "pull6")
		assert.NotContains(t, pageBody(t, "/org/org3/issues"), "issue6")
		assert.NotContains(t, pageBody(t, "/milestones"), milestone.Name)

		resp := MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/org3/repo3/activities/feeds").AddTokenAuth(token), http.StatusOK)
		for _, activity := range *DecodeJSON(t, resp, &[]*api.Activity{}) {
			assert.NotEqual(t, "create_issue", activity.OpType)
		}
		assert.NotContains(t, pageBody(t, "/org3/repo3.rss"), "<item>")
	})

	t.Run("IssuesOnly", func(t *testing.T) {
		setTeamUnits(t, unit.TypeIssues)
		assert.Contains(t, searchTitles(t, "&type=issues"), "issue6")
		assert.NotContains(t, searchTitles(t, ""), "pull6")

		session.MakeRequest(t, NewRequest(t, "GET", "/org3/repo3/issues/search?type=pulls"), http.StatusNotFound)
		assert.NotContains(t, pageBody(t, "/org3/repo3/issues/search?state=all"), "pull6")
		assert.NotContains(t, pageBody(t, fmt.Sprintf("/org3/repo3/milestone/%d", milestone.ID)), "pull6")
	})

	t.Run("PullsOnly", func(t *testing.T) {
		setTeamUnits(t, unit.TypePullRequests)
		session.MakeRequest(t, NewRequest(t, "GET", "/org3/repo3/issues/posters"), http.StatusNotFound)
		assert.NotContains(t, pageBody(t, fmt.Sprintf("/org3/repo3/milestone/%d", milestone.ID)), "issue6")
	})

	t.Run("PublicRepoWithIssuesDisabled", func(t *testing.T) {
		repoUnit := unittest.AssertExistsAndLoadBean(t, &repo_model.RepoUnit{RepoID: 1, Type: unit.TypeIssues})
		_, err := db.DeleteByID[repo_model.RepoUnit](t.Context(), repoUnit.ID)
		require.NoError(t, err)
		resp := MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/issues/search?type=issues&state=all"), http.StatusOK)
		for _, issue := range *DecodeJSON(t, resp, &[]*api.Issue{}) {
			assert.NotEqual(t, "user2/repo1", issue.Repo.FullName)
		}
	})
}
