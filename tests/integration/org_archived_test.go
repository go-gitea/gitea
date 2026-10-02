// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/organization"
	"gitea.dev/models/unittest"
	api "gitea.dev/modules/structs"
	org_service "gitea.dev/services/org"
	"gitea.dev/tests"

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

func archiveOrg3(t *testing.T) *organization.Organization {
	t.Helper()
	org3 := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
	require.NoError(t, org_service.SetOrganizationArchived(t.Context(), org3, true))
	return org3
}

func TestArchivedOrgProjectsReadOnly(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	archiveOrg3(t)

	token := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteIssue, auth_model.AccessTokenScopeWriteOrganization)
	MakeRequest(t, NewRequest(t, "GET", "/api/v1/orgs/org3/projects").AddTokenAuth(token), http.StatusOK)
	req := NewRequestWithJSON(t, "POST", "/api/v1/orgs/org3/projects", &api.CreateProjectOption{Title: "new"}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusLocked)
	req = NewRequestWithJSON(t, "PATCH", "/api/v1/orgs/org3/projects/16", &api.EditProjectOption{Title: new("renamed")}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusLocked)

	session := loginUser(t, "user2")
	resp := session.MakeRequest(t, NewRequest(t, "GET", "/org3/-/projects"), http.StatusOK)
	assert.Zero(t, NewHTMLParser(t, resp.Body).Find(`a[href$="/projects/new"]`).Length())
	session.MakeRequest(t, NewRequestWithValues(t, "POST", "/org3/-/projects/new", map[string]string{"title": "new"}), http.StatusForbidden)
	session.MakeRequest(t, NewRequest(t, "POST", "/org3/-/projects/16/close"), http.StatusForbidden)
}

func TestArchivedOrgSettingsReadOnly(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	archiveOrg3(t)

	token := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteOrganization, auth_model.AccessTokenScopeWriteIssue)
	MakeRequest(t, NewRequest(t, "GET", "/api/v1/orgs/org3/labels").AddTokenAuth(token), http.StatusOK)
	for _, req := range []*RequestWrapper{
		NewRequestWithJSON(t, "POST", "/api/v1/orgs/org3/labels", &api.CreateLabelOption{Name: "l", Color: "#123456"}),
		NewRequestWithJSON(t, "POST", "/api/v1/orgs/org3/hooks", &api.CreateHookOption{Type: "gitea", Config: api.CreateHookOptionConfig{"url": "http://example.com", "content_type": "json"}}),
		NewRequestWithJSON(t, "PUT", "/api/v1/orgs/org3/actions/secrets/S", &api.CreateOrUpdateSecretOption{Data: "x"}),
		NewRequestWithJSON(t, "PATCH", "/api/v1/orgs/org3", &api.EditOrgOption{Description: new("changed")}),
	} {
		MakeRequest(t, req.AddTokenAuth(token), http.StatusLocked)
	}

	session := loginUser(t, "user2")
	resp := session.MakeRequest(t, NewRequest(t, "GET", "/org/org3/settings"), http.StatusOK)
	assert.Contains(t, resp.Body.String(), "This organization is archived. Its settings are read-only")
	session.MakeRequest(t, NewRequestWithValues(t, "POST", "/org/org3/settings", map[string]string{"name": "org3", "full_name": "x"}), http.StatusForbidden)
	session.MakeRequest(t, NewRequestWithValues(t, "POST", "/org/org3/settings/labels/new", map[string]string{"title": "l", "color": "#123456"}), http.StatusForbidden)

	for page, selector := range map[string]string{
		"/org/org3/settings":                 `form[action$="/avatar"]`,
		"/org/org3/settings/labels":          ".new-label.button",
		"/org/org3/settings/hooks":           `a[href$="/hooks/gitea/new"]`,
		"/org/org3/settings/actions/secrets": `[data-modal="#add-secret-modal"]`,
	} {
		resp = session.MakeRequest(t, NewRequest(t, "GET", page), http.StatusOK)
		assert.Zero(t, NewHTMLParser(t, resp.Body).Find(selector).Length(), page)
	}

	req := NewRequestWithJSON(t, "PATCH", "/api/v1/orgs/org3", &api.EditOrgOption{Archived: new(false)}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK)
	assert.False(t, unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3}).IsArchived)
}

func TestArchivedOrgPackagesReadOnly(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	url := "/api/packages/org3/generic/pkg/1.0.0/file.bin"
	req := NewRequestWithBody(t, "PUT", url, strings.NewReader("a")).AddBasicAuth("user2")
	MakeRequest(t, req, http.StatusCreated)

	archiveOrg3(t)
	req = NewRequestWithBody(t, "PUT", "/api/packages/org3/generic/pkg/1.0.1/file.bin", strings.NewReader("b")).AddBasicAuth("user2")
	MakeRequest(t, req, http.StatusUnauthorized)
	MakeRequest(t, NewRequest(t, "DELETE", url).AddBasicAuth("user2"), http.StatusUnauthorized)
	MakeRequest(t, NewRequest(t, "GET", url).AddBasicAuth("user2"), http.StatusOK)
}
