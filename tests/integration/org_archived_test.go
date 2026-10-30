// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"strconv"
	"strings"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	"gitea.dev/models/organization"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArchivedOrg(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteOrganization,
		auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteIssue)
	pkgURL := "/api/packages/org3/generic/pkg/1.0.0/file.bin"
	MakeRequest(t, NewRequestWithBody(t, "PUT", pkgURL, strings.NewReader("a")).AddBasicAuth("user2"), http.StatusCreated)
	req := NewRequestWithJSON(t, "POST", "/api/v1/orgs/org3/hooks", &api.CreateHookOption{
		Type: "gitea", Config: api.CreateHookOptionConfig{"url": "http://example.com", "content_type": "json"},
	}).AddTokenAuth(token)
	hookID := strconv.FormatInt(DecodeJSON(t, MakeRequest(t, req, http.StatusCreated), &api.Hook{}).ID, 10)

	t.Run("MirrorsRefused", func(t *testing.T) {
		const errMsg = "This organization has mirror repositories (repo5). Convert them to regular repositories before archiving."
		resp := session.MakeRequest(t, NewRequest(t, "GET", "/org/org3/settings"), http.StatusOK)
		assert.Contains(t, resp.Body.String(), errMsg)
		assert.True(t, NewHTMLParser(t, resp.Body).Find(`button[data-url$="/archive?archive=true"]`).HasClass("disabled"))

		resp = session.MakeRequest(t, NewRequest(t, "POST", "/org/org3/settings/archive?archive=true"), http.StatusBadRequest)
		assert.Contains(t, resp.Body.String(), errMsg)

		req := NewRequestWithJSON(t, "PATCH", "/api/v1/orgs/org3", &api.EditOrgOption{Archived: new(true), Description: new("changed")}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusUnprocessableEntity)
		org3 := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
		assert.False(t, org3.IsArchived)
		assert.NotEqual(t, "changed", org3.Description)
	})

	_, err := db.GetEngine(t.Context()).ID(5).Cols("is_mirror").Update(&repo_model.Repository{IsMirror: false})
	require.NoError(t, err)
	session.MakeRequest(t, NewRequest(t, "POST", "/org/org3/settings/archive?archive=true"), http.StatusOK)
	assert.True(t, unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3}).IsArchived)
	unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3, IsArchived: true})

	t.Run("CreateAndTransferIn", func(t *testing.T) {
		req := NewRequestWithValues(t, "POST", "/user27/template1/fork", map[string]string{"uid": "3", "repo_name": "forked"})
		resp := session.MakeRequest(t, req, http.StatusBadRequest)
		assert.Contains(t, resp.Body.String(), "Cannot create repositories in an archived organization.")

		req = NewRequestWithJSON(t, "POST", "/api/v1/repos/user27/template1/generate", &api.GenerateRepoOption{
			Owner: "org3", Name: "generated", GitContent: true,
		}).AddTokenAuth(getUserToken(t, "user1", auth_model.AccessTokenScopeWriteRepository))
		MakeRequest(t, req, http.StatusForbidden)

		req = NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo1/transfer", &api.TransferRepoOption{NewOwner: "org3"}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusAccepted)
		unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1, OwnerID: 3, IsArchived: true})
	})

	t.Run("RepoUnarchiveBlocked", func(t *testing.T) {
		req := NewRequestWithJSON(t, "PATCH", "/api/v1/repos/org3/repo3", &api.EditRepoOption{Archived: new(false)}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusUnprocessableEntity)
		session.MakeRequest(t, NewRequestWithValues(t, "POST", "/org3/repo3/settings", map[string]string{"action": "unarchive"}), http.StatusSeeOther)
		unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3, IsArchived: true})

		resp := session.MakeRequest(t, NewRequest(t, "GET", "/org3/repo3/settings"), http.StatusOK)
		assert.True(t, NewHTMLParser(t, resp.Body).Find(`button[data-modal="#archive-repo-modal"]`).HasClass("disabled"))
		resp = session.MakeRequest(t, NewRequest(t, "GET", "/org3"), http.StatusOK)
		assert.Zero(t, NewHTMLParser(t, resp.Body).Find(`a[href$="/repo/create?org=3"]`).Length())
	})

	t.Run("ReadOnly", func(t *testing.T) {
		MakeRequest(t, NewRequest(t, "GET", "/api/v1/orgs/org3/labels").AddTokenAuth(token), http.StatusOK)
		for _, req := range []*RequestWrapper{
			NewRequestWithJSON(t, "POST", "/api/v1/orgs/org3/projects", &api.CreateProjectOption{Title: "new"}),
			NewRequestWithJSON(t, "PATCH", "/api/v1/orgs/org3/projects/16", &api.EditProjectOption{Title: new("renamed")}),
			NewRequestWithJSON(t, "POST", "/api/v1/orgs/org3/labels", &api.CreateLabelOption{Name: "l", Color: "#123456"}),
			NewRequestWithJSON(t, "DELETE", "/api/v1/orgs/org3/hooks/"+hookID, nil),
			NewRequestWithJSON(t, "PUT", "/api/v1/orgs/org3/actions/secrets/S", &api.CreateOrUpdateSecretOption{Data: "x"}),
			NewRequestWithJSON(t, "PATCH", "/api/v1/orgs/org3", &api.EditOrgOption{Description: new("changed")}),
		} {
			MakeRequest(t, req.AddTokenAuth(token), http.StatusLocked)
		}
		MakeRequest(t, NewRequest(t, "POST", "/api/v1/orgs/org3/actions/runners/registration-token"), http.StatusUnauthorized)

		for _, req := range []*RequestWrapper{
			NewRequestWithValues(t, "POST", "/org/org3/settings", map[string]string{"name": "org3", "full_name": "x"}),
			NewRequestWithValues(t, "POST", "/org/org3/settings/labels/new", map[string]string{"title": "l", "color": "#123456"}),
			NewRequestWithValues(t, "POST", "/org3/-/projects/new", map[string]string{"title": "new"}),
			NewRequest(t, "POST", "/org3/-/projects/16/close"),
		} {
			session.MakeRequest(t, req, http.StatusForbidden)
		}

		MakeRequest(t, NewRequestWithBody(t, "PUT", "/api/packages/org3/generic/pkg/1.0.1/file.bin", strings.NewReader("b")).AddBasicAuth("user2"), http.StatusUnauthorized)
		MakeRequest(t, NewRequest(t, "DELETE", pkgURL).AddBasicAuth("user2"), http.StatusUnauthorized)
		MakeRequest(t, NewRequest(t, "GET", pkgURL).AddBasicAuth("user2"), http.StatusOK)

		resp := session.MakeRequest(t, NewRequest(t, "GET", "/org/org3/settings"), http.StatusOK)
		assert.Contains(t, resp.Body.String(), "This organization is archived. Its settings are read-only")
		for page, selector := range map[string]string{
			"/org/org3/settings":                 `form[action$="/avatar"]`,
			"/org/org3/settings/labels":          ".new-label.button",
			"/org/org3/settings/hooks":           `a[href$="/hooks/gitea/new"]`,
			"/org/org3/settings/hooks/" + hookID: `[data-url$="/delete?id=` + hookID + `"]`,
			"/org/org3/settings/actions/secrets": `[data-modal="#add-secret-modal"]`,
			"/org3/-/projects":                   `a[href$="/projects/new"]`,
		} {
			resp := session.MakeRequest(t, NewRequest(t, "GET", page), http.StatusOK)
			assert.Zero(t, NewHTMLParser(t, resp.Body).Find(selector).Length(), page)
		}
		for _, page := range []string{
			"/org/org3/settings",
			"/org/org3/settings/hooks/" + hookID,
			"/org/org3/settings/actions/runners/34347",
			"/org/org3/settings/actions/general",
			"/org/org3/settings/packages/rules/add",
			"/org3/-/projects/new",
		} {
			resp := session.MakeRequest(t, NewRequest(t, "GET", page), http.StatusOK)
			assert.NotZero(t, NewHTMLParser(t, resp.Body).Find("fieldset[disabled]").Length(), page)
		}
	})

	req = NewRequestWithJSON(t, "PATCH", "/api/v1/orgs/org3", &api.EditOrgOption{Archived: new(false)}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK)
	assert.False(t, unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3}).IsArchived)
	unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3, IsArchived: true})
}
