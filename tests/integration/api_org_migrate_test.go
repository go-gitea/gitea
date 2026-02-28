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

	// Test that migration requires authentication
	t.Run("NoAuth", func(t *testing.T) {
		req := NewRequestWithJSON(t, "POST", "/api/v1/orgs/migrate", &api.MigrateOrgOptions{
			CloneAddr:     "https://github.com",
			SourceOrgName: "test-org",
			TargetOrgName: "org3",
			Service:       structs.GithubService,
		})
		MakeRequest(t, req, http.StatusUnauthorized)
	})

	// Test that target org must exist
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

	// Test that user must be org owner
	// Test that user must be org owner (user4 is in org3 but not an owner)
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

	// Test successful migration request (will fail at actual migration due to network, but validates API)
	// Test that a valid request is accepted (migration will fail at network, but API contract is validated)
	t.Run("ValidRequest", func(t *testing.T) {
		// user2 is owner of org3
		session := loginUser(t, "user2")
		token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteOrganization, auth_model.AccessTokenScopeWriteRepository)

		// Verify org3 exists
		org := unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: "org3", Type: user_model.UserTypeOrganization})
		assert.NotNil(t, org)

		// Make the request - migration will fail at the network level (can't reach github.com in tests),
		// so we expect either success (201) or an internal error (500), not a client error (4xx).
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

		// If we got a successful response, verify the structure
		if resp.Code == http.StatusCreated {
			var result api.OrgMigrationResult
			DecodeJSON(t, resp, &result)
			assert.GreaterOrEqual(t, result.TotalRepos, 0)
		}
	})
}

func TestWebOrgMigrate(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// Test that the migration page requires login
	t.Run("RequiresLogin", func(t *testing.T) {
		req := NewRequest(t, "GET", "/org/migrate")
		MakeRequest(t, req, http.StatusSeeOther) // Redirect to login
	})

	// Test that logged-in user can access migration page
	t.Run("PageLoads", func(t *testing.T) {
		session := loginUser(t, "user1")
		req := NewRequest(t, "GET", "/org/migrate")
		resp := session.MakeRequest(t, req, http.StatusOK)

		// Verify page contains expected elements
		htmlDoc := NewHTMLParser(t, resp.Body)
		AssertHTMLElement(t, htmlDoc, "form.ui.form", true)
		AssertHTMLElement(t, htmlDoc, "#clone_addr", true)
		AssertHTMLElement(t, htmlDoc, "#source_org_name", true)
		AssertHTMLElement(t, htmlDoc, "#target_org_name", true)
	})

	// Test form submission with missing required fields re-renders the form
	t.Run("MissingFields", func(t *testing.T) {
		session := loginUser(t, "user1")

		// Get real CSRF token first
		req := NewRequest(t, "GET", "/org/migrate")
		resp := session.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		csrf := htmlDoc.GetInputValueByName("_csrf")

		// Submit with only CSRF but missing required fields; form re-renders with validation errors (200)
		req = NewRequestWithValues(t, "POST", "/org/migrate", map[string]string{
			"_csrf": csrf,
		})
		session.MakeRequest(t, req, http.StatusOK)
	})

	// Test form submission with valid data structure
	t.Run("ValidFormSubmission", func(t *testing.T) {
		session := loginUser(t, "user1")

		// Get the form page first to extract CSRF
		req := NewRequest(t, "GET", "/org/migrate")
		resp := session.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		csrf := htmlDoc.GetInputValueByName("_csrf")

		// Get the orgs available for selection
		_ = unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: "org3", Type: user_model.UserTypeOrganization})

		// Submit the form
		form := map[string]string{
			"_csrf":           csrf,
			"clone_addr":      "https://github.com",
			"source_org_name": "go-gitea",
			"target_org_name": "org3",
			"service":         "2", // GitHub
			"auth_token":      "",
		}

		req = NewRequestWithValues(t, "POST", "/org/migrate", form)
		// Migration may fail at network level but should not be a server error
		resp = session.MakeRequest(t, req, NoExpectedStatus)
		assert.NotEqual(t, http.StatusInternalServerError, resp.Code)
	})
}

func TestAPIOrgMigrateServiceTypes(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user1")
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteOrganization, auth_model.AccessTokenScopeWriteRepository)

	// Test each supported service type
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

			// We just verify the service type is not rejected as invalid (4xx validation error).
			// Actual migration will fail at the network level.
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
