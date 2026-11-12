// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

// TestAPIGetCurrentToken tests getting metadata of the currently authenticated token
func TestAPIGetCurrentToken(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	for _, testCase := range []struct {
		name, tokenName string
		scope           auth_model.AccessTokenScope
	}{
		{"Success with all scopes", "test-get-current-token-all", auth_model.AccessTokenScopeAll},
		{"Success with limited scopes", "test-get-current-token-limited", auth_model.AccessTokenScopeReadRepository},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
			accessToken := createAPIAccessTokenWithoutCleanUp(t, testCase.tokenName, user, []auth_model.AccessTokenScope{testCase.scope})

			req := NewRequest(t, "GET", "/api/v1/token").
				AddTokenAuth(accessToken.Token)
			resp := MakeRequest(t, req, http.StatusOK)

			currentToken := DecodeJSON(t, resp, &api.CurrentAccessToken{})
			assert.Equal(t, accessToken.ID, currentToken.ID)
			assert.Equal(t, accessToken.Name, currentToken.Name)
			assert.Equal(t, user.ID, currentToken.User.ID)
			assert.Equal(t, user.Name, currentToken.User.Login)
		})
	}

	t.Run("Bad token", func(t *testing.T) {
		req := NewRequest(t, "GET", "/api/v1/token").
			AddTokenAuth("this does not exist")
		MakeRequest(t, req, http.StatusUnauthorized)

		req = NewRequest(t, "GET", "/api/v1/token")
		MakeRequest(t, req, http.StatusUnauthorized)
	})

	// TestAPITokenSelfService tests delete operations on token
	t.Run("SelfService", func(t *testing.T) {
		t.Run("Success then verify deleted", func(t *testing.T) {
			user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
			accessToken := createAPIAccessTokenWithoutCleanUp(t, "test-delete-current-token", user, []auth_model.AccessTokenScope{auth_model.AccessTokenScopeAll})

			// Delete the token via the endpoint
			req := NewRequest(t, "DELETE", "/api/v1/token").
				AddTokenAuth(accessToken.Token)
			MakeRequest(t, req, http.StatusNoContent)

			// Verify the token is deleted
			unittest.AssertNotExistsBean(t, &auth_model.AccessToken{ID: accessToken.ID})

			// Verify the token can no longer be used for GET
			req = NewRequest(t, "GET", "/api/v1/token").
				AddTokenAuth(accessToken.Token)
			MakeRequest(t, req, http.StatusUnauthorized)

			// Verify the token can no longer be used for DELETE
			req = NewRequest(t, "DELETE", "/api/v1/token").
				AddTokenAuth(accessToken.Token)
			MakeRequest(t, req, http.StatusUnauthorized)
		})

		t.Run("Bad token", func(t *testing.T) {
			req := NewRequest(t, "DELETE", "/api/v1/token").
				AddTokenAuth("this does not exist")
			MakeRequest(t, req, http.StatusUnauthorized)

			req = NewRequest(t, "DELETE", "/api/v1/token")
			MakeRequest(t, req, http.StatusUnauthorized)
		})
	})
}
