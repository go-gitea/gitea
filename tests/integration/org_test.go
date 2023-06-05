// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestOrgRepos(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

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
				req := NewRequest(t, "GET", "/user3?sort="+sortBy)
				resp := session.MakeRequest(t, req, http.StatusOK)

				htmlDoc := NewHTMLParser(t, resp.Body)

				sel := htmlDoc.doc.Find("a.name")
				assert.Len(t, repos, len(sel.Nodes))
				for i := 0; i < len(repos); i++ {
					assert.EqualValues(t, repos[i], strings.TrimSpace(sel.Eq(i).Text()))
				}
			}
		})
	}
}

func TestLimitedOrg(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// not logged in user
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

func TestPrivateOrg(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// not logged in user
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

func TestOrgMembers(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// not logged in user
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

func TestOrgRestrictedUser(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// privated_org is a private org who has id 23
	orgName := "privated_org"

	// public_repo_on_private_org is a public repo on privated_org
	repoName := "public_repo_on_private_org"

	// user29 is a restricted user who is not a member of the organization
	restrictedUser := "user29"

	// #17003 reports a bug whereby adding a restricted user to a read-only team doesn't work

	// assert restrictedUser cannot see the org or the public repo
	restrictedSession := loginUser(t, restrictedUser)
	req := NewRequest(t, "GET", fmt.Sprintf("/%s", orgName))
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

	req = NewRequestWithJSON(t, "POST",
		fmt.Sprintf("/api/v1/orgs/%s/teams?token=%s", orgName, token), teamToCreate)

	var apiTeam api.Team

	resp := adminSession.MakeRequest(t, req, http.StatusCreated)
	DecodeJSON(t, resp, &apiTeam)
	checkTeamResponse(t, "CreateTeam_codereader", &apiTeam, teamToCreate.Name, teamToCreate.Description, teamToCreate.IncludesAllRepositories,
		teamToCreate.Permission, teamToCreate.Units, nil)
	checkTeamBean(t, apiTeam.ID, teamToCreate.Name, teamToCreate.Description, teamToCreate.IncludesAllRepositories,
		teamToCreate.Permission, teamToCreate.Units, nil)
	// teamID := apiTeam.ID

	// Now we need to add the restricted user to the team
	req = NewRequest(t, "PUT",
		fmt.Sprintf("/api/v1/teams/%d/members/%s?token=%s", apiTeam.ID, restrictedUser, token))
	_ = adminSession.MakeRequest(t, req, http.StatusNoContent)

	// Now we need to check if the restrictedUser can access the repo
	req = NewRequest(t, "GET", fmt.Sprintf("/%s", orgName))
	restrictedSession.MakeRequest(t, req, http.StatusOK)

	req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s", orgName, repoName))
	restrictedSession.MakeRequest(t, req, http.StatusOK)
}

func TestTeamSearch(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 15})
	org := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 17})

	var results TeamSearchResults

	session := loginUser(t, user.Name)
	csrf := GetCSRF(t, session, "/"+org.Name)
	req := NewRequestf(t, "GET", "/org/%s/teams/-/search?q=%s", org.Name, "_team")
	req.Header.Add("X-Csrf-Token", csrf)
	resp := session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &results)
	assert.NotEmpty(t, results.Data)
	assert.Len(t, results.Data, 2)
	assert.Equal(t, "review_team", results.Data[0].Name)
	assert.Equal(t, "test_team", results.Data[1].Name)

	// no access if not organization member
	user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	session = loginUser(t, user5.Name)
	csrf = GetCSRF(t, session, "/"+org.Name)
	req = NewRequestf(t, "GET", "/org/%s/teams/-/search?q=%s", org.Name, "team")
	req.Header.Add("X-Csrf-Token", csrf)
	session.MakeRequest(t, req, http.StatusNotFound)
}
