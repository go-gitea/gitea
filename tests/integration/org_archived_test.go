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
	"gitea.dev/models/db"
	"gitea.dev/models/organization"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	api "gitea.dev/modules/structs"
	org_service "gitea.dev/services/org"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func archiveOrg3(t *testing.T) *organization.Organization {
	t.Helper()
	_, err := db.GetEngine(t.Context()).ID(5).Cols("is_mirror").Update(&repo_model.Repository{IsMirror: false})
	require.NoError(t, err)
	org3 := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
	require.NoError(t, org_service.SetOrganizationArchived(t.Context(), org3, true))
	return org3
}

func TestArchiveOrgWithMirrorsRefused(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	const errMsg = "This organization has mirror repositories (repo5). Convert them to regular repositories before archiving."

	session := loginUser(t, "user2")
	resp := session.MakeRequest(t, NewRequest(t, "GET", "/org/org3/settings"), http.StatusOK)
	assert.Contains(t, resp.Body.String(), errMsg)
	assert.True(t, NewHTMLParser(t, resp.Body).Find(`button[data-url$="/archive?archive=true"]`).HasClass("disabled"))

	resp = session.MakeRequest(t, NewRequest(t, "POST", "/org/org3/settings/archive?archive=true"), http.StatusBadRequest)
	assert.Contains(t, resp.Body.String(), errMsg)

	token := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteOrganization)
	req := NewRequestWithJSON(t, "PATCH", "/api/v1/orgs/org3", &api.EditOrgOption{Archived: new(true), Description: new("changed")}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusUnprocessableEntity)
	org3 := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
	assert.False(t, org3.IsArchived)
	assert.NotEqual(t, "changed", org3.Description)
}

func TestCreateRepoInArchivedOrg(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		org3 := archiveOrg3(t)

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

func TestArchivedOrgSettingsDetailPagesReadOnly(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	token := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteOrganization)
	req := NewRequestWithJSON(t, "POST", "/api/v1/orgs/org3/hooks", &api.CreateHookOption{
		Type: "gitea", Config: api.CreateHookOptionConfig{"url": "http://example.com", "content_type": "json"},
	}).AddTokenAuth(token)
	hook := DecodeJSON(t, MakeRequest(t, req, http.StatusCreated), &api.Hook{})
	archiveOrg3(t)

	session := loginUser(t, "user2")
	for page, selector := range map[string]string{
		"/org/org3/settings/hooks/" + strconv.FormatInt(hook.ID, 10): `[data-url$="/delete?id=` + strconv.FormatInt(hook.ID, 10) + `"]`,
		"/org/org3/settings/actions/runners/34347":                   `[data-url$="/delete"]`,
	} {
		resp := session.MakeRequest(t, NewRequest(t, "GET", page), http.StatusOK)
		assert.Zero(t, NewHTMLParser(t, resp.Body).Find(selector).Length(), page)
	}

	for _, page := range []string{
		"/org/org3/settings",
		"/org/org3/settings/hooks/" + strconv.FormatInt(hook.ID, 10),
		"/org/org3/settings/hooks/gitea/new",
		"/org/org3/settings/actions/runners/34347",
		"/org/org3/settings/actions/general",
		"/org/org3/settings/packages/rules/add",
		"/org3/-/projects/new",
	} {
		resp := session.MakeRequest(t, NewRequest(t, "GET", page), http.StatusOK)
		doc := NewHTMLParser(t, resp.Body)
		assert.NotZero(t, doc.Find("fieldset[disabled]").Length(), page)
		if strings.Contains(page, "/hooks/") {
			assert.True(t, doc.Find(".ui.right.type.dropdown").HasClass("disabled"), page)
		}
	}
}
