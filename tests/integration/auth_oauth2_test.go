// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/services/auth/source/oauth2"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOIDCIgnoresStaleExternalLoginLinks(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&setting.OAuth2Client.EnableAutoRegistration, true)()
	defer test.MockVariableValue(&setting.OAuth2Client.AccountLinking, setting.OAuth2AccountLinkingAuto)()
	defer test.MockVariableValue(&setting.OAuth2Client.Username, setting.OAuth2UsernameEmail)()

	setup := func(t *testing.T, sourceName, sub, userName, email string) (*auth_model.Source, *user_model.User) {
		t.Helper()
		srv := newFakeOIDCServerWithProfile(t, sub, sub+"-oid", email, "OIDC Test User")
		addOAuth2Source(t, sourceName, oauth2.Source{
			Provider:                      "openidConnect",
			ClientID:                      "test-client-id",
			ClientSecret:                  "test-client-secret",
			OpenIDConnectAutoDiscoveryURL: srv.URL + "/.well-known/openid-configuration",
		})
		authSource, err := auth_model.GetActiveOAuth2SourceByAuthName(t.Context(), sourceName)
		require.NoError(t, err)
		correctUser := &user_model.User{Name: userName, Email: email}
		require.NoError(t, user_model.CreateUser(t.Context(), correctUser, &user_model.Meta{}))
		return authSource, correctUser
	}

	// assertRelinked signs in via OIDC and asserts the stale link now points at the correct individual user.
	assertRelinked := func(t *testing.T, authSource *auth_model.Source, sub string, correctUser *user_model.User) {
		t.Helper()
		doOIDCSignIn(t, authSource.Name)
		// external_login_user has no "id" column, so order by the primary key instead
		externalLink := unittest.AssertExistsAndLoadBean(t, &user_model.ExternalLoginUser{ExternalID: sub, LoginSourceID: authSource.ID}, unittest.OrderBy("external_id ASC"))
		assert.Equal(t, correctUser.ID, externalLink.UserID)
		assert.Equal(t, correctUser.Email, externalLink.Email)
		assert.Equal(t, "OIDC Test User", externalLink.Name)
	}

	t.Run("organization", func(t *testing.T) {
		const sub, userName, email = "oidc-stale-org-link-sub", "guizar_m", "guizar_m@example.com"
		authSource, correctUser := setup(t, "test-oidc-stale-org-link", sub, userName, email)
		org := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3, Type: user_model.UserTypeOrganization})
		require.NoError(t, user_model.LinkExternalToUser(t.Context(), org, &user_model.ExternalLoginUser{
			ExternalID:    sub,
			UserID:        org.ID,
			LoginSourceID: authSource.ID,
			Provider:      authSource.Name,
		}))
		assertRelinked(t, authSource, sub, correctUser)
	})

	t.Run("deleted user", func(t *testing.T) {
		const sub, userName, email = "oidc-stale-deleted-link-sub", "guizar_d", "guizar_d@example.com"
		const deletedUserID = 999999
		authSource, correctUser := setup(t, "test-oidc-stale-deleted", sub, userName, email)
		// link the external account to a user id that does not exist, simulating a deleted user
		require.NoError(t, user_model.LinkExternalToUser(t.Context(), &user_model.User{ID: deletedUserID}, &user_model.ExternalLoginUser{
			ExternalID:    sub,
			UserID:        deletedUserID,
			LoginSourceID: authSource.ID,
			Provider:      authSource.Name,
		}))
		assertRelinked(t, authSource, sub, correctUser)
	})
}

func newFakeOIDCServerWithProfile(t *testing.T, sub, oid, email, name string) *httptest.Server {
	t.Helper()

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration": // discovery document
			_ = json.NewEncoder(w).Encode(map[string]string{
				"issuer":                 srv.URL,
				"authorization_endpoint": srv.URL + "/authorize",
				"token_endpoint":         srv.URL + "/token",
				"userinfo_endpoint":      srv.URL + "/userinfo",
			})
		case "/token": // returns an ID token with both "sub" and "oid" claims so tests can verify which one ends up as ExternalID
			claims := map[string]any{
				"iss": srv.URL,
				"aud": "test-client-id",
				"exp": time.Now().Add(time.Hour).Unix(),
				"sub": sub,
				"oid": oid,
			}
			payload, _ := json.Marshal(claims)
			header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))

			// build a JWT-shaped string whose payload encodes claims.
			// goth's decodeJWT only base64-decodes the payload without verifying the signature, so no real signing infrastructure is needed.
			idToken := header + "." + base64.RawURLEncoding.EncodeToString(payload) + ".fakesig"

			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "fake-access-token",
				"token_type":   "Bearer",
				"id_token":     idToken,
			})
		case "/userinfo":
			// sub MUST match the id_token sub; goth rejects mismatches.
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sub":   sub,
				"email": email,
				"name":  name,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// doOIDCSignIn runs a mock OIDC sign-in flow for the given auth source.
func doOIDCSignIn(t *testing.T, sourceName string) {
	t.Helper()
	session := emptyTestSession(t)

	// Step 1: initiate login
	resp := session.MakeRequest(t, NewRequest(t, "GET", "/user/oauth2/"+sourceName), http.StatusTemporaryRedirect)

	// Step 2: extract the UUID state that Gitea embedded in the redirect URL.
	location := resp.Header().Get("Location")
	u, err := url.Parse(location)
	require.NoError(t, err)
	state := u.Query().Get("state")
	require.NotEmpty(t, state, "redirect to OIDC provider must include state")

	// Step 3: simulate the provider redirecting back.
	callbackURL := fmt.Sprintf("/user/oauth2/%s/callback?code=test-code&state=%s", sourceName, url.QueryEscape(state))
	session.MakeRequest(t, NewRequest(t, "GET", callbackURL), http.StatusSeeOther)
}
