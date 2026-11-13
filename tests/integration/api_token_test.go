// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/log"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPITokens(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// TestAPIGetTokensPermission ensures that only the admin can get tokens from other users
	t.Run("GetTokensPermission", func(t *testing.T) {
		// admin can get tokens for other users
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
		req := NewRequest(t, "GET", "/api/v1/users/user2/tokens").
			AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusOK)

		// non-admin can get tokens for himself
		user = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		req = NewRequest(t, "GET", "/api/v1/users/user2/tokens").
			AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusOK)

		// non-admin can't get tokens for other users
		user = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
		req = NewRequest(t, "GET", "/api/v1/users/user2/tokens").
			AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusForbidden)
	})

	// TestAPIDeleteMissingToken ensures that error is thrown when token not found
	t.Run("DeleteMissingToken", func(t *testing.T) {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

		req := NewRequestf(t, "DELETE", "/api/v1/users/user1/tokens/%d", unittest.NonexistentID).
			AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusNotFound)
	})

	// TestAPICreateAndDeleteToken tests that token that was just created can be deleted
	t.Run("CreateAndDeleteToken", func(t *testing.T) {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

		newAccessToken := createAPIAccessTokenWithoutCleanUp(t, "test-key-1", user, []auth_model.AccessTokenScope{auth_model.AccessTokenScopeAll})
		deleteAPIAccessToken(t, newAccessToken, user)

		newAccessToken = createAPIAccessTokenWithoutCleanUp(t, "test-key-2", user, []auth_model.AccessTokenScope{auth_model.AccessTokenScopeAll})
		deleteAPIAccessToken(t, newAccessToken, user)
	})

	// TestAPIDeniesPermissionBasedOnTokenScope tests that API routes forbid access
	// when the correct token scope is not included.
	t.Run("DeniesPermissionBasedOnTokenScope", func(t *testing.T) {
		// We'll assert that each endpoint, when fetched with a token with all
		// scopes *except* the ones specified, a forbidden status code is returned.
		//
		// This is to protect against endpoints having their access check copied
		// from other endpoints and not updated.
		//
		// Test cases are in alphabetical order by URL.
		type route struct {
			url, method string
			level       auth_model.AccessTokenScopeLevel
		}
		testCases := []struct {
			categories []auth_model.AccessTokenScopeCategory
			routes     []route
		}{
			{
				[]auth_model.AccessTokenScopeCategory{auth_model.AccessTokenScopeCategoryAdmin},
				[]route{
					{"/api/v1/admin/emails", "GET", auth_model.Read},
					{"/api/v1/admin/users", "GET", auth_model.Read},
					{"/api/v1/admin/users", "POST", auth_model.Write},
					{"/api/v1/admin/users/user2", "PATCH", auth_model.Write},
					{"/api/v1/admin/users/user2/orgs", "GET", auth_model.Read},
					{"/api/v1/admin/users/user2/orgs", "POST", auth_model.Write},
					{"/api/v1/admin/orgs", "GET", auth_model.Read},
				},
			},
			{
				[]auth_model.AccessTokenScopeCategory{auth_model.AccessTokenScopeCategoryNotification},
				[]route{
					{"/api/v1/notifications", "GET", auth_model.Read},
					{"/api/v1/notifications", "PUT", auth_model.Write},
				},
			},
			{
				[]auth_model.AccessTokenScopeCategory{auth_model.AccessTokenScopeCategoryOrganization, auth_model.AccessTokenScopeCategoryRepository},
				[]route{
					{"/api/v1/org/org1/repos", "POST", auth_model.Write},
				},
			},
			{
				[]auth_model.AccessTokenScopeCategory{auth_model.AccessTokenScopeCategoryPackage},
				[]route{
					{"/api/v1/packages/user1/type/name/1", "GET", auth_model.Read},
					{"/api/v1/packages/user1/type/name/1", "DELETE", auth_model.Write},
				},
			},
			{
				[]auth_model.AccessTokenScopeCategory{auth_model.AccessTokenScopeCategoryRepository},
				[]route{
					{"/api/v1/repos/user1/repo1", "GET", auth_model.Read},
					{"/api/v1/repos/user1/repo1", "PATCH", auth_model.Write},
					{"/api/v1/repos/user1/repo1", "DELETE", auth_model.Write},
					{"/api/v1/repos/user1/repo1/branches", "GET", auth_model.Read},
					{"/api/v1/repos/user1/repo1/archive/foo", "GET", auth_model.Read},
				},
			},
			{
				[]auth_model.AccessTokenScopeCategory{auth_model.AccessTokenScopeCategoryIssue},
				[]route{
					{"/api/v1/repos/user1/repo1/issues", "GET", auth_model.Read},
				},
			},
			{
				[]auth_model.AccessTokenScopeCategory{auth_model.AccessTokenScopeCategoryRepository},
				[]route{
					{"/api/v1/repos/user1/repo1/media/foo", "GET", auth_model.Read},
					{"/api/v1/repos/user1/repo1/raw/foo", "GET", auth_model.Read},
					{"/api/v1/repos/user1/repo1/teams", "GET", auth_model.Read},
					{"/api/v1/repos/user1/repo1/teams/team1", "PUT", auth_model.Write},
					{"/api/v1/repos/user1/repo1/transfer", "POST", auth_model.Write},
					// Private repo
					{"/api/v1/repos/user2/repo2", "GET", auth_model.Read},
					// Private repo
					{"/api/v1/repos/user2/repo2", "GET", auth_model.Read},
				},
			},
			{
				[]auth_model.AccessTokenScopeCategory{auth_model.AccessTokenScopeCategoryUser},
				[]route{
					{"/api/v1/user", "GET", auth_model.Read},
					{"/api/v1/user/emails", "GET", auth_model.Read},
					{"/api/v1/user/emails", "POST", auth_model.Write},
					{"/api/v1/user/emails", "DELETE", auth_model.Write},
					{"/api/v1/user/applications/oauth2", "GET", auth_model.Read},
					{"/api/v1/user/applications/oauth2", "POST", auth_model.Write},
					{"/api/v1/users/search", "GET", auth_model.Read},
					// Private user
					{"/api/v1/users/user31", "GET", auth_model.Read},
					// Private user
					{"/api/v1/users/user31/gpg_keys", "GET", auth_model.Read},
				},
			},
		}

		// User needs to be admin so that we can verify that tokens without admin
		// scopes correctly deny access.
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
		assert.True(t, user.IsAdmin, "User needs to be admin")

		for _, testCase := range testCases {
			for _, route := range testCase.routes {
				requiredPermissions := make([]permission, len(testCase.categories))
				for i, category := range testCase.categories {
					requiredPermissions[i] = permission{category, route.level}
				}
				runTestCase(t, &requiredScopeTestCase{route.url, route.method, requiredPermissions}, user)
			}
		}
	})

	// TestAPIDeleteTokensPermission ensures that only the admin can delete tokens from other users
	t.Run("DeleteTokensPermission", func(t *testing.T) {
		admin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})

		// admin can delete tokens for other users
		createAPIAccessTokenWithoutCleanUp(t, "test-key-1", user2, []auth_model.AccessTokenScope{auth_model.AccessTokenScopeAll})
		req := NewRequest(t, "DELETE", "/api/v1/users/"+user2.LoginName+"/tokens/test-key-1").
			AddBasicAuth(admin.Name)
		MakeRequest(t, req, http.StatusNoContent)

		// non-admin can delete tokens for himself
		createAPIAccessTokenWithoutCleanUp(t, "test-key-2", user2, []auth_model.AccessTokenScope{auth_model.AccessTokenScopeAll})
		req = NewRequest(t, "DELETE", "/api/v1/users/"+user2.LoginName+"/tokens/test-key-2").
			AddBasicAuth(user2.Name)
		MakeRequest(t, req, http.StatusNoContent)

		// non-admin can't delete tokens for other users
		createAPIAccessTokenWithoutCleanUp(t, "test-key-3", user2, []auth_model.AccessTokenScope{auth_model.AccessTokenScopeAll})
		req = NewRequest(t, "DELETE", "/api/v1/users/"+user2.LoginName+"/tokens/test-key-3").
			AddBasicAuth(user4.Name)
		MakeRequest(t, req, http.StatusForbidden)
	})

	// TestAPICreateTokenScopeEscalation ensures a token-authenticated request cannot
	// mint a new token with a broader scope than the authenticating token.
	t.Run("CreateTokenScopeEscalation", func(t *testing.T) {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

		// a write:user-scoped token authenticates the create requests below
		writeUserToken := getUserToken(t, user.Name, auth_model.AccessTokenScopeWriteUser)

		// requesting a broader scope ("all") than the authenticating token is rejected
		req := NewRequestWithJSON(t, "POST", "/api/v1/users/"+user.LoginName+"/tokens", map[string]any{
			"name":   "escalated",
			"scopes": []string{"all"},
		})
		req.Request.SetBasicAuth(user.Name, writeUserToken)
		MakeRequest(t, req, http.StatusForbidden)

		// requesting a subset scope ("read:user") is allowed
		req = NewRequestWithJSON(t, "POST", "/api/v1/users/"+user.LoginName+"/tokens", map[string]any{
			"name":   "subset",
			"scopes": []string{"read:user"},
		})
		req.Request.SetBasicAuth(user.Name, writeUserToken)
		MakeRequest(t, req, http.StatusCreated)

		// password (non-token) auth may still create a token with any scope
		req = NewRequestWithJSON(t, "POST", "/api/v1/users/"+user.LoginName+"/tokens", map[string]any{
			"name":   "by-password",
			"scopes": []string{"all"},
		}).AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusCreated)

		// a public-only token must not mint a token that drops the public-only restriction
		publicOnlyToken := getUserToken(t, user.Name, auth_model.AccessTokenScopeWriteUser, auth_model.AccessTokenScopePublicOnly)
		req = NewRequestWithJSON(t, "POST", "/api/v1/users/"+user.LoginName+"/tokens", map[string]any{
			"name":   "still-public-only",
			"scopes": []string{"write:user"},
		})
		req.Request.SetBasicAuth(user.Name, publicOnlyToken)
		resp := MakeRequest(t, req, http.StatusCreated)
		var createdToken api.AccessToken
		DecodeJSON(t, resp, &createdToken)
		assert.Contains(t, createdToken.Scopes, string(auth_model.AccessTokenScopePublicOnly))

		// an unrestricted parent token may create a narrower public-only child: public-only is a restriction,
		// not a grantable permission, so the subset check must not reject it
		req = NewRequestWithJSON(t, "POST", "/api/v1/users/"+user.LoginName+"/tokens", map[string]any{
			"name":   "narrower-public-only",
			"scopes": []string{"write:user", "public-only"},
		})
		req.Request.SetBasicAuth(user.Name, writeUserToken)
		resp = MakeRequest(t, req, http.StatusCreated)
		DecodeJSON(t, resp, &createdToken)
		assert.Contains(t, createdToken.Scopes, string(auth_model.AccessTokenScopePublicOnly))
	})
}

type permission struct {
	category auth_model.AccessTokenScopeCategory
	level    auth_model.AccessTokenScopeLevel
}

type requiredScopeTestCase struct {
	url                 string
	method              string
	requiredPermissions []permission
}

func (c *requiredScopeTestCase) Name() string {
	return fmt.Sprintf("%v %v", c.method, c.url)
}

// runTestCase Helper function to run a single test case.
func runTestCase(t *testing.T, testCase *requiredScopeTestCase, user *user_model.User) {
	t.Run(testCase.Name(), func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		// Create a token with all scopes NOT required by the endpoint.
		var unauthorizedScopes []auth_model.AccessTokenScope
		for _, category := range auth_model.AllAccessTokenScopeCategories {
			// For permissions, Write > Read > NoAccess.  So we need to
			// find the minimum required, and only grant permission up to but
			// not including the minimum required.
			minRequiredLevel := auth_model.Write
			categoryIsRequired := false
			for _, requiredPermission := range testCase.requiredPermissions {
				if requiredPermission.category != category {
					continue
				}
				categoryIsRequired = true
				if requiredPermission.level < minRequiredLevel {
					minRequiredLevel = requiredPermission.level
				}
			}
			unauthorizedLevel := auth_model.Write
			if categoryIsRequired {
				switch minRequiredLevel {
				case auth_model.Read:
					unauthorizedLevel = auth_model.NoAccess
				case auth_model.Write:
					unauthorizedLevel = auth_model.Read
				default:
					assert.FailNow(t, "Invalid test case", "Unknown access token scope level: %v", minRequiredLevel)
				}
			}

			if unauthorizedLevel == auth_model.NoAccess {
				continue
			}
			categoryUnauthorizedScopes := auth_model.GetRequiredScopes(
				unauthorizedLevel,
				category)
			unauthorizedScopes = append(unauthorizedScopes, categoryUnauthorizedScopes...)
		}

		accessToken := createAPIAccessTokenWithoutCleanUp(t, "test-token", user, unauthorizedScopes)
		defer deleteAPIAccessToken(t, accessToken, user)

		// Request the endpoint.  Verify that permission is denied.
		req := NewRequest(t, testCase.method, testCase.url).
			AddTokenAuth(accessToken.Token)
		MakeRequest(t, req, http.StatusForbidden)
	})
}

// createAPIAccessTokenWithoutCleanUp Create an API access token and assert that
// creation succeeded.  The caller is responsible for deleting the token.
func createAPIAccessTokenWithoutCleanUp(t *testing.T, tokenName string, user *user_model.User, scopes []auth_model.AccessTokenScope) api.AccessToken {
	payload := map[string]any{
		"name":   tokenName,
		"scopes": scopes,
	}

	log.Debug("Requesting creation of token with scopes: %v", scopes)
	req := NewRequestWithJSON(t, "POST", "/api/v1/users/"+user.LoginName+"/tokens", payload).
		AddBasicAuth(user.Name)
	resp := MakeRequest(t, req, http.StatusCreated)

	newAccessToken := DecodeJSON(t, resp, &api.AccessToken{})
	unittest.AssertExistsAndLoadBean(t, &auth_model.AccessToken{
		ID:    newAccessToken.ID,
		Name:  newAccessToken.Name,
		Token: newAccessToken.Token,
		UID:   user.ID,
	})

	return *newAccessToken
}

// deleteAPIAccessToken deletes an API access token and assert that deletion succeeded.
func deleteAPIAccessToken(t *testing.T, accessToken api.AccessToken, user *user_model.User) {
	req := NewRequestf(t, "DELETE", "/api/v1/users/"+user.LoginName+"/tokens/%d", accessToken.ID).
		AddBasicAuth(user.Name)
	MakeRequest(t, req, http.StatusNoContent)

	unittest.AssertNotExistsBean(t, &auth_model.AccessToken{ID: accessToken.ID})
}
