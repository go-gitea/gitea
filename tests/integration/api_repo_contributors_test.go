// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/organization"
	"gitea.dev/models/perm"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"
	org_service "gitea.dev/services/org"
	repo_service "gitea.dev/services/repository"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIRepoContributors(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	token := getUserToken(t, "user2", auth_model.AccessTokenScopeReadRepository)

	listContributors := func(t *testing.T, link string) ([]*api.Contributor, string) {
		// the first request queues the counting, which the test queues run right away
		MakeRequest(t, NewRequest(t, "GET", link).AddTokenAuth(token), http.StatusAccepted)
		resp := MakeRequest(t, NewRequest(t, "GET", link).AddTokenAuth(token), http.StatusOK)
		return DecodeJSON(t, resp, []*api.Contributor{}), resp.Header().Get("X-Total-Count")
	}

	t.Run("Users", func(t *testing.T) {
		contributors, total := listContributors(t, "/api/v1/repos/user2/repo16/contributors")
		assert.Equal(t, "2", total)
		if assert.Len(t, contributors, 2) {
			assert.Equal(t, "user2", contributors[0].Login)
			assert.Equal(t, &api.Contributor{
				Type:          "User",
				ID:            21,
				Login:         "user21",
				AvatarURL:     contributors[1].AvatarURL,
				HTMLURL:       contributors[1].HTMLURL,
				Contributions: 1,
				Additions:     1,
				Deletions:     1,
			}, contributors[1])
		}
	})

	t.Run("Anonymous", func(t *testing.T) {
		contributors, total := listContributors(t, "/api/v1/repos/user2/repo2/contributors?anon=true&limit=1")
		assert.Equal(t, "3", total)
		assert.Equal(t, []*api.Contributor{{
			Type:          "Anonymous",
			Name:          "Ethan Koenig",
			Email:         "ethantkoenig@gmail.com",
			Contributions: 1,
			Additions:     3,
		}}, contributors)

		resp := MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo2/contributors").AddTokenAuth(token), http.StatusOK)
		assert.Equal(t, "0", resp.Header().Get("X-Total-Count"))
	})

	t.Run("NoAccess", func(t *testing.T) {
		MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo16/contributors"), http.StatusNotFound)
	})

	t.Run("IssuesOnlyAccess", func(t *testing.T) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3})
		team := &organization.Team{OrgID: repo.OwnerID, Name: "issues-only", Units: []*organization.TeamUnit{
			{OrgID: repo.OwnerID, Type: unit.TypeIssues, AccessMode: perm.AccessModeRead},
		}}
		require.NoError(t, org_service.NewTeam(t.Context(), team))
		require.NoError(t, org_service.AddTeamMember(t.Context(), team, unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})))
		require.NoError(t, repo_service.TeamAddRepository(t.Context(), team, repo))

		user5Token := getUserToken(t, "user5", auth_model.AccessTokenScopeReadRepository, auth_model.AccessTokenScopeReadIssue)
		MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/org3/repo3/issues").AddTokenAuth(user5Token), http.StatusOK)
		MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/org3/repo3/contributors").AddTokenAuth(user5Token), http.StatusForbidden)
	})
}
