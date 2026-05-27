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

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/json"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/services/auth/source/oauth2"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMigrateAzureADV2ToOIDC simulates a login source migration from the Azure AD V2 OAuth2 provider to the OpenID Connect provider,
// and verifies that setting ExternalIDClaim = "oid" restores account continuity.
//
// Background: Azure AD V2 (goth's azureadv2 provider) fetches the user profile from Microsoft Graph API (/v1.0/me)
// and uses the "id" field - the stable Object ID (OID) - as gothUser.UserID. That OID is stored as ExternalID in external_login_user.
//
// When the admin migrates the same source to OpenID Connect, the goth openidConnect provider defaults to ["sub"] for UserIdClaims.
// Azure AD's "sub" is pairwise (unique per application), so it differs from the OID that was previously stored,
// causing every existing user to appear as a new account.
//
// Setting ExternalIDClaim = "oid" on the OIDC source overrides UserIdClaims to ["oid"],
// so the same OID is extracted and matched against the existing rows, restoring continuity.
func TestMigrateAzureADV2ToOIDC(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&setting.OAuth2Client.EnableAutoRegistration, true)()
	// Use UserID (gothUser.UserID) as the Gitea username so that different ExternalID values produce different, non-conflicting usernames.
	defer test.MockVariableValue(&setting.OAuth2Client.Username, setting.OAuth2UsernameUserid)()

	const (
		sourceName = "test-migrate-azure"

		// oidValue is the stable Azure AD Object ID, used as ExternalID by the Azure AD V2 provider.
		oidValue = "oid-object-id-stable"

		// subValue is the pairwise sub issued by Azure AD for OpenID Connect; it differs from oidValue and would produce a separate account if used.
		subValue = "sub-pairwise-value"
	)

	// The fake OIDC server issues tokens containing both sub and oid claims, mirroring what Azure AD v2.0 returns.
	srv := newFakeOIDCServer(t, subValue, oidValue)

	// --- Step 1: Establish the legacy Azure AD V2 state ---
	// Create an azureadv2 auth source. In production this would have been the source used before the migration.
	addOAuth2Source(t, sourceName, oauth2.Source{
		Provider:     "azureadv2",
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		CustomURLMapping: &oauth2.CustomURLMapping{
			Tenant: "test-tenant-id",
		},
	})
	authSource, err := auth_model.GetActiveOAuth2SourceByAuthName(t.Context(), sourceName)
	require.NoError(t, err)

	// Create a user to represent the "legacy" account that was originally registered through the Azure AD V2 provider.
	legacyUser := &user_model.User{
		Name:  "legacy-azure-user",
		Email: "legacy-azure-user@example.com",
	}
	require.NoError(t, user_model.CreateUser(t.Context(), legacyUser, &user_model.Meta{}))
	require.NoError(t, user_model.LinkExternalToUser(t.Context(), legacyUser, &user_model.ExternalLoginUser{
		ExternalID:    oidValue,
		UserID:        legacyUser.ID,
		LoginSourceID: authSource.ID,
		Provider:      authSource.Name,
	}))

	// --- Step 2: Migrate the source to OIDC without ExternalIDClaim ---
	// The provider type of the OAuth2 source is changed from azureadv2 to openidConnect.
	// Without ExternalIDClaim the goth provider defaults to "sub", which does not match the stored OID, so every sign-in creates a fresh account.
	authSource.Cfg = &oauth2.Source{
		Provider:                      "openidConnect",
		ClientID:                      "test-client-id",
		ClientSecret:                  "test-client-secret",
		OpenIDConnectAutoDiscoveryURL: srv.URL + "/.well-known/openid-configuration",
		// ExternalIDClaim intentionally not set; goth defaults to "sub".
	}
	err = auth_model.UpdateSource(t.Context(), authSource)
	require.NoError(t, err)

	t.Run("without ExternalIDClaim: legacy user is NOT matched", func(t *testing.T) {
		// Confirm the external user with ExternalID=subValue doesn't exist.
		unittest.AssertNotExistsBean(t, &user_model.ExternalLoginUser{ExternalID: subValue, LoginSourceID: authSource.ID}, unittest.OrderBy("external_id ASC"))

		doOIDCSignIn(t, sourceName)

		// "sub" is now the ExternalID - a new user was auto-registered.
		subEntry := unittest.AssertExistsAndLoadBean(t, &user_model.ExternalLoginUser{ExternalID: subValue, LoginSourceID: authSource.ID}, unittest.OrderBy("external_id ASC"))
		// The auto-registered user is NOT the legacy user.
		assert.NotEqual(t, legacyUser.ID, subEntry.UserID)
	})

	// --- Step 3: Set ExternalIDClaim = "oid" to restore account continuity ---
	// Set ExternalIDClaim = "oid" so that the OIDC source extracts the same Object ID that the Azure AD V2 provider previously stored.
	authSource.Cfg.(*oauth2.Source).ExternalIDClaim = "oid"
	err = auth_model.UpdateSource(t.Context(), authSource)
	require.NoError(t, err)

	t.Run("with ExternalIDClaim=oid: legacy user IS matched", func(t *testing.T) {
		// Confirm the legacy oid row has no RawData yet - it was created directly via LinkExternalToUser in setup, without going through an OAuth flow.
		oidEntry := unittest.AssertExistsAndLoadBean(t, &user_model.ExternalLoginUser{ExternalID: oidValue, LoginSourceID: authSource.ID}, unittest.OrderBy("external_id ASC"))
		require.Nil(t, oidEntry.RawData)

		doOIDCSignIn(t, sourceName)

		// After sign-in, RawData should contain both "oid" and "name".
		oidEntry = unittest.AssertExistsAndLoadBean(t, &user_model.ExternalLoginUser{ExternalID: oidValue, LoginSourceID: authSource.ID}, unittest.OrderBy("external_id ASC"))
		assert.Equal(t, oidValue, oidEntry.RawData["oid"])
		assert.Equal(t, "OIDC Test User", oidEntry.RawData["name"])

		// The matched user must still be the original legacy user.
		assert.Equal(t, legacyUser.ID, oidEntry.UserID)
	})
}

// newFakeOIDCServer starts an httptest.Server that implements the minimum OIDC endpoints needed to complete a sign-in flow:
func newFakeOIDCServer(t *testing.T, sub, oid string) *httptest.Server {
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
				"email": sub + "@example.com",
				"name":  "OIDC Test User",
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

// newFakeOIDCServerWithGroups starts an httptest.Server that implements OIDC endpoints
// with configurable group claims in both ID token and userinfo responses.
func newFakeOIDCServerWithGroups(t *testing.T, sub, email, name string, groups []string) *httptest.Server {
	t.Helper()

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"issuer":                 srv.URL,
				"authorization_endpoint": srv.URL + "/authorize",
				"token_endpoint":         srv.URL + "/token",
				"userinfo_endpoint":      srv.URL + "/userinfo",
			})
		case "/token":
			claims := map[string]any{
				"iss":   srv.URL,
				"aud":   "test-client-id",
				"exp":   time.Now().Add(time.Hour).Unix(),
				"sub":   sub,
				"email": email,
				"name":  name,
			}
			if groups != nil {
				claims["groups"] = groups
			}
			payload, _ := json.Marshal(claims)
			header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
			idToken := header + "." + base64.RawURLEncoding.EncodeToString(payload) + ".fakesig"

			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "fake-access-token",
				"token_type":   "Bearer",
				"id_token":     idToken,
			})
		case "/userinfo":
			response := map[string]any{
				"sub":   sub,
				"email": email,
				"name":  name,
			}
			if groups != nil {
				response["groups"] = groups
			}
			_ = json.NewEncoder(w).Encode(response)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestOAuth2GroupClaimsAppliedOnFirstLogin verifies that group claims from OAuth2/OIDC
// are correctly applied to newly created users on first login, not just on subsequent logins.
// This tests the fix for the issue where restricted/admin flags were only set on second login.
func TestOAuth2GroupClaimsAppliedOnFirstLogin(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	// Enable auto-registration to ensure first login creates user with group claims
	defer test.MockVariableValue(&setting.OAuth2Client.EnableAutoRegistration, true)()
	// Use sub claim as username for deterministic user naming
	defer test.MockVariableValue(&setting.OAuth2Client.Username, setting.OAuth2UsernameUserid)()

	// Setup OIDC server with group claims
	srv := newFakeOIDCServerWithGroups(t, "user123", "user123@example.com", "Test User",
		[]string{"developers", "admins", "restricted-users"})

	sourceName := "test-group-claims"
	addOAuth2Source(t, sourceName, oauth2.Source{
		Provider:                      "openidConnect",
		ClientID:                      "test-client-id",
		ClientSecret:                  "test-client-secret",
		OpenIDConnectAutoDiscoveryURL: srv.URL + "/.well-known/openid-configuration",
		GroupClaimName:                "groups",
		AdminGroup:                    "admins",
		RestrictedGroup:               "restricted-users",
	})

	t.Run("user in both admin and restricted groups", func(t *testing.T) {
		defer tests.PrepareTestEnv(t)()
		defer test.MockVariableValue(&setting.OAuth2Client.EnableAutoRegistration, true)()
		defer test.MockVariableValue(&setting.OAuth2Client.Username, setting.OAuth2UsernameUserid)()

		// Use a fresh server with groups
		srv := newFakeOIDCServerWithGroups(t, "user-both-groups", "both@example.com", "Both Groups User",
			[]string{"developers", "admins", "restricted-users"})
		addOAuth2Source(t, "test-both", oauth2.Source{
			Provider:                      "openidConnect",
			ClientID:                      "test-client-id",
			ClientSecret:                  "test-client-secret",
			OpenIDConnectAutoDiscoveryURL: srv.URL + "/.well-known/openid-configuration",
			GroupClaimName:                "groups",
			AdminGroup:                    "admins",
			RestrictedGroup:               "restricted-users",
		})

		doOIDCSignIn(t, "test-both")

		// Verify user was created with correct group claim flags
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user-both-groups"})
		assert.True(t, user.IsAdmin, "User should be admin from 'admins' group claim")
		assert.True(t, user.IsRestricted, "User should be restricted from 'restricted-users' group claim")
		assert.Equal(t, auth_model.OAuth2, user.LoginType)
	})

	t.Run("user in admin group only", func(t *testing.T) {
		defer tests.PrepareTestEnv(t)()
		defer test.MockVariableValue(&setting.OAuth2Client.EnableAutoRegistration, true)()
		defer test.MockVariableValue(&setting.OAuth2Client.Username, setting.OAuth2UsernameUserid)()

		srv := newFakeOIDCServerWithGroups(t, "user-admin-only", "admin@example.com", "Admin Only User",
			[]string{"developers", "admins"})
		addOAuth2Source(t, "test-admin-only", oauth2.Source{
			Provider:                      "openidConnect",
			ClientID:                      "test-client-id",
			ClientSecret:                  "test-client-secret",
			OpenIDConnectAutoDiscoveryURL: srv.URL + "/.well-known/openid-configuration",
			GroupClaimName:                "groups",
			AdminGroup:                    "admins",
			RestrictedGroup:               "restricted-users",
		})

		doOIDCSignIn(t, "test-admin-only")

		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user-admin-only"})
		assert.True(t, user.IsAdmin, "User should be admin from 'admins' group claim")
		assert.False(t, user.IsRestricted, "User should NOT be restricted (not in restricted group)")
	})

	t.Run("user in restricted group only", func(t *testing.T) {
		defer tests.PrepareTestEnv(t)()
		defer test.MockVariableValue(&setting.OAuth2Client.EnableAutoRegistration, true)()
		defer test.MockVariableValue(&setting.OAuth2Client.Username, setting.OAuth2UsernameUserid)()

		srv := newFakeOIDCServerWithGroups(t, "user-restricted-only", "restricted@example.com", "Restricted Only User",
			[]string{"developers", "restricted-users"})
		addOAuth2Source(t, "test-restricted-only", oauth2.Source{
			Provider:                      "openidConnect",
			ClientID:                      "test-client-id",
			ClientSecret:                  "test-client-secret",
			OpenIDConnectAutoDiscoveryURL: srv.URL + "/.well-known/openid-configuration",
			GroupClaimName:                "groups",
			AdminGroup:                    "admins",
			RestrictedGroup:               "restricted-users",
		})

		doOIDCSignIn(t, "test-restricted-only")

		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user-restricted-only"})
		assert.False(t, user.IsAdmin, "User should NOT be admin (not in admin group)")
		assert.True(t, user.IsRestricted, "User should be restricted from 'restricted-users' group claim")
	})

	t.Run("user in neither group", func(t *testing.T) {
		defer tests.PrepareTestEnv(t)()
		defer test.MockVariableValue(&setting.OAuth2Client.EnableAutoRegistration, true)()
		defer test.MockVariableValue(&setting.OAuth2Client.Username, setting.OAuth2UsernameUserid)()

		srv := newFakeOIDCServerWithGroups(t, "user-neither-group", "neither@example.com", "Neither Group User",
			[]string{"developers"})
		addOAuth2Source(t, "test-neither", oauth2.Source{
			Provider:                      "openidConnect",
			ClientID:                      "test-client-id",
			ClientSecret:                  "test-client-secret",
			OpenIDConnectAutoDiscoveryURL: srv.URL + "/.well-known/openid-configuration",
			GroupClaimName:                "groups",
			AdminGroup:                    "admins",
			RestrictedGroup:               "restricted-users",
		})

		doOIDCSignIn(t, "test-neither")

		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user-neither-group"})
		assert.False(t, user.IsAdmin, "User should NOT be admin (not in admin group)")
		assert.Equal(t, setting.Service.DefaultUserIsRestricted, user.IsRestricted,
			"User should use global default (not in restricted group)")
	})

	t.Run("user with no groups claim", func(t *testing.T) {
		defer tests.PrepareTestEnv(t)()
		defer test.MockVariableValue(&setting.OAuth2Client.EnableAutoRegistration, true)()
		defer test.MockVariableValue(&setting.OAuth2Client.Username, setting.OAuth2UsernameUserid)()

		// Server returns no groups claim
		srv := newFakeOIDCServerWithGroups(t, "user-no-groups", "nogroups@example.com", "No Groups User", nil)
		addOAuth2Source(t, "test-no-groups", oauth2.Source{
			Provider:                      "openidConnect",
			ClientID:                      "test-client-id",
			ClientSecret:                  "test-client-secret",
			OpenIDConnectAutoDiscoveryURL: srv.URL + "/.well-known/openid-configuration",
			GroupClaimName:                "groups",
			AdminGroup:                    "admins",
			RestrictedGroup:               "restricted-users",
		})

		doOIDCSignIn(t, "test-no-groups")

		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user-no-groups"})
		assert.False(t, user.IsAdmin, "User should NOT be admin (no groups claim)")
		assert.Equal(t, setting.Service.DefaultUserIsRestricted, user.IsRestricted,
			"User should use global default (no groups claim)")
	})
}

// TestOAuth2GroupClaimsManualLinking tests that group claims are applied correctly
// when a user goes through the manual linking flow (auto-registration disabled).
// This covers the path where the user is shown the link_account page and creates a new account.
func TestOAuth2GroupClaimsManualLinking(t *testing.T) {
	t.Run("manual linking with new account creation", func(t *testing.T) {
		defer tests.PrepareTestEnv(t)()
		// Disable auto-registration to force manual linking flow
		defer test.MockVariableValue(&setting.OAuth2Client.EnableAutoRegistration, false)()
		defer test.MockVariableValue(&setting.Service.AllowOnlyInternalRegistration, false)()

		// Setup OIDC server with group claims - user is in both admin and restricted groups
		srv := newFakeOIDCServerWithGroups(t, "manual-user", "manual@example.com", "Manual User",
			[]string{"developers", "admins", "restricted-users"})

		sourceName := "test-manual-linking"
		addOAuth2Source(t, sourceName, oauth2.Source{
			Provider:                      "openidConnect",
			ClientID:                      "test-client-id",
			ClientSecret:                  "test-client-secret",
			OpenIDConnectAutoDiscoveryURL: srv.URL + "/.well-known/openid-configuration",
			GroupClaimName:                "groups",
			AdminGroup:                    "admins",
			RestrictedGroup:               "restricted-users",
		})

		// Step 1: Initiate OAuth2 login - will redirect to link_account page
		session := emptyTestSession(t)
		resp := session.MakeRequest(t, NewRequest(t, "GET", "/user/oauth2/"+sourceName), http.StatusTemporaryRedirect)

		// Step 2: Extract state and simulate callback
		location := resp.Header().Get("Location")
		u, err := url.Parse(location)
		require.NoError(t, err)
		state := u.Query().Get("state")
		require.NotEmpty(t, state, "redirect to OIDC provider must include state")

		callbackURL := fmt.Sprintf("/user/oauth2/%s/callback?code=test-code&state=%s", sourceName, url.QueryEscape(state))
		session.MakeRequest(t, NewRequest(t, "GET", callbackURL), http.StatusSeeOther)

		// Step 3: Now we should be on the link_account page
		// Submit the form to create a new account
		linkAccountResp := session.MakeRequest(t, NewRequest(t, "GET", "/user/link_account"), http.StatusOK)
		// Verify we're on the link account page
		assert.Contains(t, linkAccountResp.Body.String(), "link_account")

		// Step 4: Submit the registration form
		// Use NewRequestWithValues to POST form data (no CSRF needed in tests)
		// Field names are lowercase in HTML forms: user_name, email, password, retype
		req := NewRequestWithValues(t, "POST", "/user/link_account_signup", map[string]string{
			"user_name": "manual-user",
			"email":     "manual@example.com",
			"password":  "", // AllowOnlyExternalRegistration means no password needed
			"retype":    "",
		})
		session.MakeRequest(t, req, http.StatusSeeOther)

		// Step 5: Verify user was created with correct group claim flags
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "manual-user"})
		assert.True(t, user.IsAdmin, "User should be admin from 'admins' group claim in manual linking")
		assert.True(t, user.IsRestricted, "User should be restricted from 'restricted-users' group claim in manual linking")
		assert.Equal(t, auth_model.OAuth2, user.LoginType)
	})

	t.Run("manual linking to existing account", func(t *testing.T) {
		defer tests.PrepareTestEnv(t)()
		defer test.MockVariableValue(&setting.OAuth2Client.EnableAutoRegistration, false)()
		defer test.MockVariableValue(&setting.Service.AllowOnlyInternalRegistration, false)()
		defer test.MockVariableValue(&setting.OAuth2Client.Username, setting.OAuth2UsernameUserid)()

		// Setup OIDC server with group claims - user is in admin group
		srv := newFakeOIDCServerWithGroups(t, "existing-user", "existing@example.com", "Existing User",
			[]string{"restricted-users", "admins"})

		sourceName := "test-manual-linking-existing"
		addOAuth2Source(t, sourceName, oauth2.Source{
			Provider:                      "openidConnect",
			ClientID:                      "test-client-id",
			ClientSecret:                  "test-client-secret",
			OpenIDConnectAutoDiscoveryURL: srv.URL + "/.well-known/openid-configuration",
			GroupClaimName:                "groups",
			AdminGroup:                    "admins",
			RestrictedGroup:               "restricted-users",
		})

		// Create an existing user first
		existingUser := &user_model.User{
			Name:   "existing-user",
			Email:  "existing@example.com",
			Passwd: "password",
		}
		require.NoError(t, user_model.CreateUser(t.Context(), existingUser, &user_model.Meta{}))
		// Initially not admin
		assert.False(t, existingUser.IsAdmin)

		// Step 1: Initiate OAuth2 login
		session := emptyTestSession(t)
		resp := session.MakeRequest(t, NewRequest(t, "GET", "/user/oauth2/"+sourceName), http.StatusTemporaryRedirect)

		// Step 2: Simulate callback
		location := resp.Header().Get("Location")
		u, err := url.Parse(location)
		require.NoError(t, err)
		state := u.Query().Get("state")
		require.NotEmpty(t, state, "redirect to OIDC provider must include state")

		callbackURL := fmt.Sprintf("/user/oauth2/%s/callback?code=test-code&state=%s", sourceName, url.QueryEscape(state))
		session.MakeRequest(t, NewRequest(t, "GET", callbackURL), http.StatusSeeOther)

		// Step 3: Submit the sign-in form to link to existing account
		// Use NewRequestWithValues to POST form data (no CSRF needed in tests)
		// Field names must match SignInForm struct: UserName, Password, Remember
		session.MakeRequest(t, NewRequestWithValues(t, "POST", "/user/link_account_signin", map[string]string{
			"user_name": "existing-user",
			"password":  "password",
			"Remember":  "false",
		}), http.StatusSeeOther)

		// Step 4: Verify the existing user now has admin flag from OAuth2 group claims
		// Note: This may require a follow-up login or the flags might be updated on the linking
		updatedUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "existing-user"})
		assert.True(t, updatedUser.IsAdmin, "Existing user should be admin from 'admins' group claim after linking")
	})
}
