// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/tests"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrg(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	t.Run("OrgRepos", testOrgRepos)
	t.Run("PrivateOrg", testPrivateOrg)
	t.Run("LimitedOrg", testLimitedOrg)
	t.Run("OrgMembers", testOrgMembers)
	t.Run("OrgRestrictedUser", testOrgRestrictedUser)
	t.Run("TeamSearch", testTeamSearch)
	t.Run("TeamsPage", testTeamsPage)
	t.Run("OrgSettings", testOrgSettings)
}

func testOrgRepos(t *testing.T) {
	var (
		users = []string{"user1", "user2"}
		cases = map[string][]string{
			"alphabetically":        {"repo21", "repo3", "repo5"},
			"reversealphabetically": {"repo5", "repo3", "repo21"},
		}
	)

	for _, user := range users {
		t.Run(user, func(t *testing.T) {
			session := loginUser(t, user)
			for sortBy, repos := range cases {
				req := NewRequest(t, "GET", "/org3?sort="+sortBy)
				resp := session.MakeRequest(t, req, http.StatusOK)

				htmlDoc := NewHTMLParser(t, resp.Body)

				sel := htmlDoc.doc.Find("a.name")
				assert.Len(t, repos, len(sel.Nodes))
				for i := range repos {
					assert.Equal(t, repos[i], strings.TrimSpace(sel.Eq(i).Text()))
				}
			}
		})
	}
}

func testLimitedOrg(t *testing.T) {
	// not logged-in user
	req := NewRequest(t, "GET", "/limited_org")
	MakeRequest(t, req, http.StatusNotFound)
	req = NewRequest(t, "GET", "/limited_org/public_repo_on_limited_org")
	MakeRequest(t, req, http.StatusNotFound)
	req = NewRequest(t, "GET", "/limited_org/private_repo_on_limited_org")
	MakeRequest(t, req, http.StatusNotFound)

	// login non-org member user
	session := loginUser(t, "user2")
	req = NewRequest(t, "GET", "/limited_org")
	session.MakeRequest(t, req, http.StatusOK)
	req = NewRequest(t, "GET", "/limited_org/public_repo_on_limited_org")
	session.MakeRequest(t, req, http.StatusOK)
	req = NewRequest(t, "GET", "/limited_org/private_repo_on_limited_org")
	session.MakeRequest(t, req, http.StatusNotFound)

	// site admin
	session = loginUser(t, "user1")
	req = NewRequest(t, "GET", "/limited_org")
	session.MakeRequest(t, req, http.StatusOK)
	req = NewRequest(t, "GET", "/limited_org/public_repo_on_limited_org")
	session.MakeRequest(t, req, http.StatusOK)
	req = NewRequest(t, "GET", "/limited_org/private_repo_on_limited_org")
	session.MakeRequest(t, req, http.StatusOK)
}

func testPrivateOrg(t *testing.T) {
	// not logged-in user
	req := NewRequest(t, "GET", "/privated_org")
	MakeRequest(t, req, http.StatusNotFound)
	req = NewRequest(t, "GET", "/privated_org/public_repo_on_private_org")
	MakeRequest(t, req, http.StatusNotFound)
	req = NewRequest(t, "GET", "/privated_org/private_repo_on_private_org")
	MakeRequest(t, req, http.StatusNotFound)

	// login non-org member user
	session := loginUser(t, "user2")
	req = NewRequest(t, "GET", "/privated_org")
	session.MakeRequest(t, req, http.StatusNotFound)
	req = NewRequest(t, "GET", "/privated_org/public_repo_on_private_org")
	session.MakeRequest(t, req, http.StatusNotFound)
	req = NewRequest(t, "GET", "/privated_org/private_repo_on_private_org")
	session.MakeRequest(t, req, http.StatusNotFound)

	// non-org member who is collaborator on repo in private org
	session = loginUser(t, "user4")
	req = NewRequest(t, "GET", "/privated_org")
	session.MakeRequest(t, req, http.StatusNotFound)
	req = NewRequest(t, "GET", "/privated_org/public_repo_on_private_org") // colab of this repo
	session.MakeRequest(t, req, http.StatusOK)
	req = NewRequest(t, "GET", "/privated_org/private_repo_on_private_org")
	session.MakeRequest(t, req, http.StatusNotFound)

	// site admin
	session = loginUser(t, "user1")
	req = NewRequest(t, "GET", "/privated_org")
	session.MakeRequest(t, req, http.StatusOK)
	req = NewRequest(t, "GET", "/privated_org/public_repo_on_private_org")
	session.MakeRequest(t, req, http.StatusOK)
	req = NewRequest(t, "GET", "/privated_org/private_repo_on_private_org")
	session.MakeRequest(t, req, http.StatusOK)
}

func testOrgMembers(t *testing.T) {
	// not logged-in user
	req := NewRequest(t, "GET", "/org/org25/members")
	MakeRequest(t, req, http.StatusOK)

	// org member
	session := loginUser(t, "user24")
	req = NewRequest(t, "GET", "/org/org25/members")
	session.MakeRequest(t, req, http.StatusOK)

	// site admin
	session = loginUser(t, "user1")
	req = NewRequest(t, "GET", "/org/org25/members")
	session.MakeRequest(t, req, http.StatusOK)
}

func testOrgRestrictedUser(t *testing.T) {
	// privated_org is a private org who has id 23
	orgName := "privated_org"

	// public_repo_on_private_org is a public repo on privated_org
	repoName := "public_repo_on_private_org"

	// user29 is a restricted user who is not a member of the organization
	restrictedUser := "user29"

	// #17003 reports a bug whereby adding a restricted user to a read-only team doesn't work

	// assert restrictedUser cannot see the org or the public repo
	restrictedSession := loginUser(t, restrictedUser)
	req := NewRequest(t, "GET", "/"+orgName)
	restrictedSession.MakeRequest(t, req, http.StatusNotFound)

	req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s", orgName, repoName))
	restrictedSession.MakeRequest(t, req, http.StatusNotFound)

	// Therefore create a read-only team
	adminSession := loginUser(t, "user1")
	token := getTokenForLoggedInUser(t, adminSession, auth_model.AccessTokenScopeWriteOrganization)

	teamToCreate := &api.CreateTeamOption{
		Name:                    "codereader",
		Description:             "Code Reader",
		IncludesAllRepositories: true,
		Permission:              "read",
		Units:                   []string{"repo.code"},
	}

	req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/orgs/%s/teams", orgName), teamToCreate).
		AddTokenAuth(token)

	var apiTeam api.Team

	resp := adminSession.MakeRequest(t, req, http.StatusCreated)
	DecodeJSON(t, resp, &apiTeam)
	checkTeamResponse(t, "CreateTeam_codereader", &apiTeam, teamToCreate.Name, teamToCreate.Description, teamToCreate.IncludesAllRepositories,
		"none", teamToCreate.Units, nil)
	checkTeamBean(t, apiTeam.ID, teamToCreate.Name, teamToCreate.Description, teamToCreate.IncludesAllRepositories,
		"none", teamToCreate.Units, nil)
	// teamID := apiTeam.ID

	// Now we need to add the restricted user to the team
	req = NewRequest(t, "PUT", fmt.Sprintf("/api/v1/teams/%d/members/%s", apiTeam.ID, restrictedUser)).
		AddTokenAuth(token)
	_ = adminSession.MakeRequest(t, req, http.StatusNoContent)

	// Now we need to check if the restrictedUser can access the repo
	req = NewRequest(t, "GET", "/"+orgName)
	restrictedSession.MakeRequest(t, req, http.StatusOK)

	req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s", orgName, repoName))
	restrictedSession.MakeRequest(t, req, http.StatusOK)
}

func testTeamSearch(t *testing.T) {
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 15})
	org := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 17})

	var results TeamSearchResults

	session := loginUser(t, user.Name)
	req := NewRequestf(t, "GET", "/org/%s/teams/-/search?q=%s", org.Name, "_team")
	resp := session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &results)
	assert.NotEmpty(t, results.Data)
	assert.Len(t, results.Data, 2)
	assert.Equal(t, "review_team", results.Data[0].Name)
	assert.Equal(t, "test_team", results.Data[1].Name)

	// no access if not organization member
	user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	session = loginUser(t, user5.Name)
	req = NewRequestf(t, "GET", "/org/%s/teams/-/search?q=%s", org.Name, "team")
	session.MakeRequest(t, req, http.StatusNotFound)

	t.Run("SearchWithPermission", func(t *testing.T) {
		ctx := t.Context()
		const testOrgID int64 = 500
		const testRepoID int64 = 2000
		testTeam := &organization.Team{OrgID: testOrgID, LowerName: "test_team", AccessMode: perm.AccessModeNone}
		require.NoError(t, db.Insert(ctx, testTeam))
		require.NoError(t, db.Insert(ctx, &organization.TeamRepo{OrgID: testOrgID, TeamID: testTeam.ID, RepoID: testRepoID}))
		require.NoError(t, db.Insert(ctx, &organization.TeamUnit{OrgID: testOrgID, TeamID: testTeam.ID, Type: unit.TypeCode, AccessMode: perm.AccessModeRead}))
		require.NoError(t, db.Insert(ctx, &organization.TeamUnit{OrgID: testOrgID, TeamID: testTeam.ID, Type: unit.TypeIssues, AccessMode: perm.AccessModeWrite}))

		teams, err := organization.GetTeamsWithAccessToAnyRepoUnit(ctx, testOrgID, testRepoID, perm.AccessModeRead, unit.TypeCode, unit.TypeIssues)
		require.NoError(t, err)
		assert.Len(t, teams, 1) // can read "code" or "issues"

		teams, err = organization.GetTeamsWithAccessToAnyRepoUnit(ctx, testOrgID, testRepoID, perm.AccessModeWrite, unit.TypeCode)
		require.NoError(t, err)
		assert.Empty(t, teams) // cannot write "code"

		teams, err = organization.GetTeamsWithAccessToAnyRepoUnit(ctx, testOrgID, testRepoID, perm.AccessModeWrite, unit.TypeIssues)
		require.NoError(t, err)
		assert.Len(t, teams, 1) // can write "issues"

		_, _ = db.GetEngine(ctx).ID(testTeam.ID).Update(&organization.Team{AccessMode: perm.AccessModeWrite})
		teams, err = organization.GetTeamsWithAccessToAnyRepoUnit(ctx, testOrgID, testRepoID, perm.AccessModeWrite, unit.TypeCode)
		require.NoError(t, err)
		assert.Len(t, teams, 1) // team permission is "write", so can write "code"
	})
}

func testTeamsPage(t *testing.T) {
	// org17 has three teams in fixtures: Owners (id 5), test_team (id 8), review_team (id 9).
	// user15 is in Owners; user20 is in review_team only; user5 is not a member.
	org := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 17})

	listTeams := func(t *testing.T, session *TestSession, query string) []string {
		req := NewRequestf(t, "GET", "/org/%s/teams%s", org.Name, query)
		resp := session.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		sel := htmlDoc.doc.Find(".ui.top.attached.header strong")
		names := make([]string, 0, sel.Length())
		sel.Each(func(_ int, s *goquery.Selection) {
			names = append(names, s.Text())
		})
		return names
	}

	// Owner sees all teams, "Owners" sorted first regardless of alphabetical order
	ownerSession := loginUser(t, "user15")
	assert.Equal(t, []string{"Owners", "review_team", "test_team"}, listTeams(t, ownerSession, ""))

	// Keyword filter narrows by name
	assert.Equal(t, []string{"review_team"}, listTeams(t, ownerSession, "?q=review"))

	// Non-admin org member sees only the teams they belong to
	memberSession := loginUser(t, "user20")
	assert.Equal(t, []string{"review_team"}, listTeams(t, memberSession, ""))

	// Edit review_team so user20 gets full list
	reviewTeam := unittest.AssertExistsAndLoadBean(t, &organization.Team{ID: 9})
	req := NewRequestWithValues(t, "POST", fmt.Sprintf("/org/%s/teams/%s/edit", org.Name, reviewTeam.Name), map[string]string{
		"team_name":   reviewTeam.Name,
		"description": reviewTeam.Description,
		"repo_access": "all",
		"permission":  "admin",
		"unit_1":      "1",
		"unit_2":      "1",
		"unit_3":      "1",
		"unit_4":      "1",
		"unit_5":      "1",
		"unit_6":      "1",
		"unit_7":      "1",
		"unit_8":      "1",
		"unit_9":      "1",
		"unit_10":     "1",
	})
	ownerSession.MakeRequest(t, req, http.StatusSeeOther)
	assert.Equal(t, []string{"Owners", "review_team", "test_team"}, listTeams(t, memberSession, ""))

	// Non-member is denied
	nonMemberSession := loginUser(t, "user5")
	req = NewRequestf(t, "GET", "/org/%s/teams", org.Name)
	nonMemberSession.MakeRequest(t, req, http.StatusNotFound)

	t.Run("Pagination", func(t *testing.T) {
		defer test.MockVariableValue(&setting.UI.MembersPagingNum, 2)()
		assert.Len(t, listTeams(t, ownerSession, "?page=1"), 2)
		assert.Equal(t, []string{"test_team"}, listTeams(t, ownerSession, "?page=2"))
	})
}

func testOrgSettings(t *testing.T) {
	session := loginUser(t, "user2")

	req := NewRequestWithValues(t, "POST", "/org/org3/settings", map[string]string{
		"full_name": "org3 new full name",
		"email":     "org3-new-email@example.com",
	})
	session.MakeRequest(t, req, http.StatusSeeOther)
	org := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})
	assert.Equal(t, "org3 new full name", org.FullName)
	assert.Equal(t, "org3-new-email@example.com", org.Email)

	req = NewRequestWithValues(t, "POST", "/org/org3/settings", map[string]string{
		"email": "", // empty email means "clear email"
	})
	session.MakeRequest(t, req, http.StatusSeeOther)
	org = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})
	assert.Equal(t, "org3 new full name", org.FullName)
	assert.Empty(t, org.Email)
}
