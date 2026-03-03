// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/structs"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIOrgMigrate(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	t.Run("NoAuth", func(t *testing.T) {
		req := NewRequestWithJSON(t, "POST", "/api/v1/orgs/migrate", &api.MigrateOrgOptions{
			CloneAddr:     "https://github.com",
			SourceOrgName: "test-org",
			TargetOrgName: "org3",
			Service:       structs.GithubService,
		})
		MakeRequest(t, req, http.StatusUnauthorized)
	})

	t.Run("TargetOrgNotExist", func(t *testing.T) {
		session := loginUser(t, "user1")
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteOrganization, auth_model.AccessTokenScopeWriteRepository)

		req := NewRequestWithJSON(t, "POST", "/api/v1/orgs/migrate", &api.MigrateOrgOptions{
			CloneAddr:     "https://github.com",
			SourceOrgName: "test-org",
			TargetOrgName: "nonexistent-org",
			Service:       structs.GithubService,
		}).AddTokenAuth(token)
		session.MakeRequest(t, req, http.StatusUnprocessableEntity)
	})

	t.Run("NotOrgOwner", func(t *testing.T) {
		session := loginUser(t, "user4")
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteOrganization, auth_model.AccessTokenScopeWriteRepository)

		req := NewRequestWithJSON(t, "POST", "/api/v1/orgs/migrate", &api.MigrateOrgOptions{
			CloneAddr:     "https://github.com",
			SourceOrgName: "test-org",
			TargetOrgName: "org3",
			Service:       structs.GithubService,
		}).AddTokenAuth(token)
		session.MakeRequest(t, req, http.StatusForbidden)
	})

	t.Run("ValidRequest", func(t *testing.T) {
		session := loginUser(t, "user2")
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteOrganization, auth_model.AccessTokenScopeWriteRepository)

		org := unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: "org3", Type: user_model.UserTypeOrganization})
		assert.NotNil(t, org)

		req := NewRequestWithJSON(t, "POST", "/api/v1/orgs/migrate", &api.MigrateOrgOptions{
			CloneAddr:     "https://github.com",
			SourceOrgName: "go-gitea",
			TargetOrgName: "org3",
			Service:       structs.GithubService,
			AuthToken:     "test-token",
			Private:       false,
			Wiki:          true,
			Issues:        false,
			PullRequests:  false,
			Releases:      true,
		}).AddTokenAuth(token)

		resp := session.MakeRequest(t, req, NoExpectedStatus)
		assert.NotEqual(t, http.StatusUnauthorized, resp.Code)
		assert.NotEqual(t, http.StatusForbidden, resp.Code)
		assert.NotEqual(t, http.StatusUnprocessableEntity, resp.Code)
		assert.NotEqual(t, http.StatusBadRequest, resp.Code)

		if resp.Code == http.StatusCreated {
			var result api.OrgMigrationResult
			DecodeJSON(t, resp, &result)
			assert.GreaterOrEqual(t, result.TotalRepos, 0)
		}
	})
}

func TestWebOrgMigrate(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	t.Run("RequiresLogin", func(t *testing.T) {
		req := NewRequest(t, "GET", "/org/migrate")
		MakeRequest(t, req, http.StatusSeeOther)
	})

	t.Run("PageLoads", func(t *testing.T) {
		session := loginUser(t, "user1")
		req := NewRequest(t, "GET", "/org/migrate")
		resp := session.MakeRequest(t, req, http.StatusOK)

		htmlDoc := NewHTMLParser(t, resp.Body)
		AssertHTMLElement(t, htmlDoc, "form.ui.form", true)
		AssertHTMLElement(t, htmlDoc, "#clone_addr", true)
		AssertHTMLElement(t, htmlDoc, "#source_org_name", true)
		AssertHTMLElement(t, htmlDoc, "#target_org_name", true)
	})

	t.Run("MissingFields", func(t *testing.T) {
		session := loginUser(t, "user1")

		req := NewRequestWithValues(t, "POST", "/org/migrate", map[string]string{})
		session.MakeRequest(t, req, http.StatusOK)
	})

	t.Run("ValidFormSubmission", func(t *testing.T) {
		session := loginUser(t, "user1")

		_ = unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: "org3", Type: user_model.UserTypeOrganization})

		req := NewRequestWithValues(t, "POST", "/org/migrate", map[string]string{
			"clone_addr":      "https://github.com",
			"source_org_name": "go-gitea",
			"target_org_name": "org3",
			"service":         "2",
			"auth_token":      "",
		})
		resp := session.MakeRequest(t, req, NoExpectedStatus)
		assert.NotEqual(t, http.StatusInternalServerError, resp.Code)
	})
}

func TestAPIOrgMigrateServiceTypes(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user1")
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteOrganization, auth_model.AccessTokenScopeWriteRepository)

	serviceTypes := []struct {
		name    string
		service structs.GitServiceType
	}{
		{"Git", structs.PlainGitService},
		{"GitHub", structs.GithubService},
		{"GitLab", structs.GitlabService},
		{"Gitea", structs.GiteaService},
		{"Gogs", structs.GogsService},
	}

	for _, st := range serviceTypes {
		t.Run(st.name, func(t *testing.T) {
			req := NewRequestWithJSON(t, "POST", "/api/v1/orgs/migrate", &api.MigrateOrgOptions{
				CloneAddr:     "https://example.com",
				SourceOrgName: "test",
				TargetOrgName: "org3",
				Service:       st.service,
			}).AddTokenAuth(token)

			resp := session.MakeRequest(t, req, NoExpectedStatus)
			assert.NotEqual(t, http.StatusUnprocessableEntity, resp.Code, "Service type %s was rejected as invalid", st.name)
			assert.NotEqual(t, http.StatusBadRequest, resp.Code, "Service type %s was rejected as invalid", st.name)
		})
	}
}

func TestAPIOrgMigrateURLValidation(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user1")
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteOrganization, auth_model.AccessTokenScopeWriteRepository)

	urlTests := []struct {
		name       string
		cloneAddr  string
		expectCode int
	}{
		{"EmptyURL", "", http.StatusUnprocessableEntity},
		{"InvalidURL", "not-a-url", http.StatusUnprocessableEntity},
		{"Localhost", "http://localhost/test", http.StatusUnprocessableEntity},
	}

	for _, tt := range urlTests {
		t.Run(tt.name, func(t *testing.T) {
			req := NewRequestWithJSON(t, "POST", "/api/v1/orgs/migrate", &api.MigrateOrgOptions{
				CloneAddr:     tt.cloneAddr,
				SourceOrgName: "test",
				TargetOrgName: "org3",
				Service:       structs.GithubService,
			}).AddTokenAuth(token)

			session.MakeRequest(t, req, tt.expectCode)
		})
	}
}
