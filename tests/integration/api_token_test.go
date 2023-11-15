// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

// TestAPICreateAndDeleteToken tests that token that was just created can be deleted
func TestAPICreateAndDeleteToken(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	newAccessToken := createAPIAccessTokenWithoutCleanUp(t, "test-key-1", user, nil)
	deleteAPIAccessToken(t, newAccessToken, user)

	newAccessToken = createAPIAccessTokenWithoutCleanUp(t, "test-key-2", user, nil)
	deleteAPIAccessToken(t, newAccessToken, user)
}

// TestAPIDeleteMissingToken ensures that error is thrown when token not found
func TestAPIDeleteMissingToken(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	req := NewRequestf(t, "DELETE", "/api/v1/users/user1/tokens/%d", unittest.NonexistentID)
	req = AddBasicAuthHeader(req, user.Name)
	MakeRequest(t, req, http.StatusNotFound)
}

// TestAPIGetTokensPermission ensures that only the admin can get tokens from other users
func TestAPIGetTokensPermission(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// admin can get tokens for other users
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	req := NewRequestf(t, "GET", "/api/v1/users/user2/tokens")
	req = AddBasicAuthHeader(req, user.Name)
	MakeRequest(t, req, http.StatusOK)

	// non-admin can get tokens for himself
	user = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	req = NewRequestf(t, "GET", "/api/v1/users/user2/tokens")
	req = AddBasicAuthHeader(req, user.Name)
	MakeRequest(t, req, http.StatusOK)

	// non-admin can't get tokens for other users
	user = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	req = NewRequestf(t, "GET", "/api/v1/users/user2/tokens")
	req = AddBasicAuthHeader(req, user.Name)
	MakeRequest(t, req, http.StatusForbidden)
}

// TestAPIDeleteTokensPermission ensures that only the admin can delete tokens from other users
func TestAPIDeleteTokensPermission(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	admin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})

	// admin can delete tokens for other users
	createAPIAccessTokenWithoutCleanUp(t, "test-key-1", user2, nil)
	req := NewRequestf(t, "DELETE", "/api/v1/users/"+user2.LoginName+"/tokens/test-key-1")
	req = AddBasicAuthHeader(req, admin.Name)
	MakeRequest(t, req, http.StatusNoContent)

	// non-admin can delete tokens for himself
	createAPIAccessTokenWithoutCleanUp(t, "test-key-2", user2, nil)
	req = NewRequestf(t, "DELETE", "/api/v1/users/"+user2.LoginName+"/tokens/test-key-2")
	req = AddBasicAuthHeader(req, user2.Name)
	MakeRequest(t, req, http.StatusNoContent)

	// non-admin can't delete tokens for other users
	createAPIAccessTokenWithoutCleanUp(t, "test-key-3", user2, nil)
	req = NewRequestf(t, "DELETE", "/api/v1/users/"+user2.LoginName+"/tokens/test-key-3")
	req = AddBasicAuthHeader(req, user4.Name)
	MakeRequest(t, req, http.StatusForbidden)
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

// TestAPIDeniesPermissionBasedOnTokenScope tests that API routes forbid access
// when the correct token scope is not included.
func TestAPIDeniesPermissionBasedOnTokenScope(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// We'll assert that each endpoint, when fetched with a token with all
	// scopes *except* the ones specified, a forbidden status code is returned.
	//
	// This is to protect against endpoints having their access check copied
	// from other endpoints and not updated.
	//
	// Test cases are in alphabetical order by URL.
	//
	// Note: query parameters are not currently supported since the token is
	// appended with `?=token=<token>`.
	testCases := []requiredScopeTestCase{
		{
			"/api/v1/admin/emails",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryAdmin,
					auth_model.Read,
				},
			},
		},
		{
			"/api/v1/admin/users",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryAdmin,
					auth_model.Read,
				},
			},
		},
		{
			"/api/v1/admin/users",
			"POST",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryAdmin,
					auth_model.Write,
				},
			},
		},
		{
			"/api/v1/admin/users/user2",
			"PATCH",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryAdmin,
					auth_model.Write,
				},
			},
		},
		{
			"/api/v1/admin/users/user2/orgs",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryAdmin,
					auth_model.Read,
				},
			},
		},
		{
			"/api/v1/admin/users/user2/orgs",
			"POST",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryAdmin,
					auth_model.Write,
				},
			},
		},
		{
			"/api/v1/admin/orgs",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryAdmin,
					auth_model.Read,
				},
			},
		},
		{
			"/api/v1/notifications",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryNotification,
					auth_model.Read,
				},
			},
		},
		{
			"/api/v1/notifications",
			"PUT",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryNotification,
					auth_model.Write,
				},
			},
		},
		{
			"/api/v1/org/org1/repos",
			"POST",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryOrganization,
					auth_model.Write,
				},
				{
					auth_model.AccessTokenScopeCategoryRepository,
					auth_model.Write,
				},
			},
		},
		{
			"/api/v1/packages/user1/type/name/1",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryPackage,
					auth_model.Read,
				},
			},
		},
		{
			"/api/v1/packages/user1/type/name/1",
			"DELETE",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryPackage,
					auth_model.Write,
				},
			},
		},
		{
			"/api/v1/repos/user1/repo1",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryRepository,
					auth_model.Read,
				},
			},
		},
		{
			"/api/v1/repos/user1/repo1",
			"PATCH",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryRepository,
					auth_model.Write,
				},
			},
		},
		{
			"/api/v1/repos/user1/repo1",
			"DELETE",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryRepository,
					auth_model.Write,
				},
			},
		},
		{
			"/api/v1/repos/user1/repo1/branches",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryRepository,
					auth_model.Read,
				},
			},
		},
		{
			"/api/v1/repos/user1/repo1/archive/foo",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryRepository,
					auth_model.Read,
				},
			},
		},
		{
			"/api/v1/repos/user1/repo1/issues",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryIssue,
					auth_model.Read,
				},
			},
		},
		{
			"/api/v1/repos/user1/repo1/media/foo",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryRepository,
					auth_model.Read,
				},
			},
		},
		{
			"/api/v1/repos/user1/repo1/raw/foo",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryRepository,
					auth_model.Read,
				},
			},
		},
		{
			"/api/v1/repos/user1/repo1/teams",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryRepository,
					auth_model.Read,
				},
			},
		},
		{
			"/api/v1/repos/user1/repo1/teams/team1",
			"PUT",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryRepository,
					auth_model.Write,
				},
			},
		},
		{
			"/api/v1/repos/user1/repo1/transfer",
			"POST",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryRepository,
					auth_model.Write,
				},
			},
		},
		// Private repo
		{
			"/api/v1/repos/user2/repo2",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryRepository,
					auth_model.Read,
				},
			},
		},
		// Private repo
		{
			"/api/v1/repos/user2/repo2",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryRepository,
					auth_model.Read,
				},
			},
		},
		{
			"/api/v1/user",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryUser,
					auth_model.Read,
				},
			},
		},
		{
			"/api/v1/user/emails",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryUser,
					auth_model.Read,
				},
			},
		},
		{
			"/api/v1/user/emails",
			"POST",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryUser,
					auth_model.Write,
				},
			},
		},
		{
			"/api/v1/user/emails",
			"DELETE",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryUser,
					auth_model.Write,
				},
			},
		},
		{
			"/api/v1/user/applications/oauth2",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryUser,
					auth_model.Read,
				},
			},
		},
		{
			"/api/v1/user/applications/oauth2",
			"POST",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryUser,
					auth_model.Write,
				},
			},
		},
		{
			"/api/v1/users/search",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryUser,
					auth_model.Read,
				},
			},
		},
		// Private user
		{
			"/api/v1/users/user31",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryUser,
					auth_model.Read,
				},
			},
		},
		// Private user
		{
			"/api/v1/users/user31/gpg_keys",
			"GET",
			[]permission{
				{
					auth_model.AccessTokenScopeCategoryUser,
					auth_model.Read,
				},
			},
		},
	}

	// User needs to be admin so that we can verify that tokens without admin
	// scopes correctly deny access.
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	assert.True(t, user.IsAdmin, "User needs to be admin")

	for _, testCase := range testCases {
		runTestCase(t, &testCase, user)
	}
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
				if minRequiredLevel == auth_model.Read {
					unauthorizedLevel = auth_model.NoAccess
				} else if minRequiredLevel == auth_model.Write {
					unauthorizedLevel = auth_model.Read
				} else {
					assert.FailNow(t, "Invalid test case: Unknown access token scope level: %v", minRequiredLevel)
				}
			}

			if unauthorizedLevel == auth_model.NoAccess {
				continue
			}
			cateogoryUnauthorizedScopes := auth_model.GetRequiredScopes(
				unauthorizedLevel,
				category)
			unauthorizedScopes = append(unauthorizedScopes, cateogoryUnauthorizedScopes...)
		}

		accessToken := createAPIAccessTokenWithoutCleanUp(t, "test-token", user, &unauthorizedScopes)
		defer deleteAPIAccessToken(t, accessToken, user)

		// Add API access token to the URL.
		url := fmt.Sprintf("%s?token=%s", testCase.url, accessToken.Token)

		// Request the endpoint.  Verify that permission is denied.
		req := NewRequestf(t, testCase.method, url)
		MakeRequest(t, req, http.StatusForbidden)
	})
}

// createAPIAccessTokenWithoutCleanUp Create an API access token and assert that
// creation succeeded.  The caller is responsible for deleting the token.
func createAPIAccessTokenWithoutCleanUp(t *testing.T, tokenName string, user *user_model.User, scopes *[]auth_model.AccessTokenScope) api.AccessToken {
	payload := map[string]any{
		"name": tokenName,
	}
	if scopes != nil {
		for _, scope := range *scopes {
			scopes, scopesExists := payload["scopes"].([]string)
			if !scopesExists {
				scopes = make([]string, 0)
			}
			scopes = append(scopes, string(scope))
			payload["scopes"] = scopes
		}
	}
	log.Debug("Requesting creation of token with scopes: %v", scopes)
	req := NewRequestWithJSON(t, "POST", "/api/v1/users/"+user.LoginName+"/tokens", payload)

	req = AddBasicAuthHeader(req, user.Name)
	resp := MakeRequest(t, req, http.StatusCreated)

	var newAccessToken api.AccessToken
	DecodeJSON(t, resp, &newAccessToken)
	unittest.AssertExistsAndLoadBean(t, &auth_model.AccessToken{
		ID:    newAccessToken.ID,
		Name:  newAccessToken.Name,
		Token: newAccessToken.Token,
		UID:   user.ID,
	})

	return newAccessToken
}

// createAPIAccessTokenWithoutCleanUp Delete an API access token and assert that
// deletion succeeded.
func deleteAPIAccessToken(t *testing.T, accessToken api.AccessToken, user *user_model.User) {
	req := NewRequestf(t, "DELETE", "/api/v1/users/"+user.LoginName+"/tokens/%d", accessToken.ID)
	req = AddBasicAuthHeader(req, user.Name)
	MakeRequest(t, req, http.StatusNoContent)

	unittest.AssertNotExistsBean(t, &auth_model.AccessToken{ID: accessToken.ID})
}
