// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"strconv"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/organization"
	"gitea.dev/models/unittest"
	api "gitea.dev/modules/structs"
	org_service "gitea.dev/services/org"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateRepoInArchivedOrg(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		org3 := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
		require.NoError(t, org_service.SetOrganizationArchived(t.Context(), org3, true))

		session := loginUser(t, "user1")
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteOrganization)
		const errMsg = "Cannot create repositories in an archived organization."

		t.Run("APIFork", func(t *testing.T) {
			req := NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo1/forks", &api.CreateForkOption{Organization: &org3.Name}).
				AddTokenAuth(token)
			MakeRequest(t, req, http.StatusForbidden)
		})

		t.Run("APIGenerate", func(t *testing.T) {
			req := NewRequestWithJSON(t, "POST", "/api/v1/repos/user27/template1/generate", &api.GenerateRepoOption{
				Owner: org3.Name, Name: "generated", GitContent: true,
			}).AddTokenAuth(token)
			MakeRequest(t, req, http.StatusForbidden)
		})

		t.Run("APIMigrate", func(t *testing.T) {
			req := NewRequestWithJSON(t, "POST", "/api/v1/repos/migrate", &api.MigrateRepoOptions{
				CloneAddr: u.String() + "user2/repo1.git", RepoOwnerID: org3.ID, RepoName: "migrated",
			}).AddTokenAuth(token)
			MakeRequest(t, req, http.StatusForbidden)
		})

		ownerSession := loginUser(t, "user2")

		t.Run("WebFork", func(t *testing.T) {
			req := NewRequestWithValues(t, "POST", "/user27/template1/fork", map[string]string{
				"uid": strconv.FormatInt(org3.ID, 10), "repo_name": "forked", "fork_single_branch": "",
			})
			resp := ownerSession.MakeRequest(t, req, http.StatusBadRequest)
			assert.Contains(t, resp.Body.String(), errMsg)
		})

		t.Run("WebMigrate", func(t *testing.T) {
			req := NewRequestWithValues(t, "POST", "/repo/migrate", map[string]string{
				"clone_addr": u.String() + "user2/repo1.git", "uid": strconv.FormatInt(org3.ID, 10),
				"repo_name": "migrated", "service": strconv.Itoa(int(api.PlainGitService)),
			})
			resp := ownerSession.MakeRequest(t, req, http.StatusOK)
			assert.Contains(t, resp.Body.String(), errMsg)
		})
	})
}
