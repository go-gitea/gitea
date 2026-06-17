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
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/json"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/services/auth/source/oauth2"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/builder"
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
	srv := newFakeOIDCServer(t, FakeOIDCConfig{Sub: subValue, OID: oidValue})

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

func TestOIDCIgnoresStaleExternalLoginLinks(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&setting.OAuth2Client.EnableAutoRegistration, true)()
	defer test.MockVariableValue(&setting.OAuth2Client.AccountLinking, setting.OAuth2AccountLinkingAuto)()
	defer test.MockVariableValue(&setting.OAuth2Client.Username, setting.OAuth2UsernameEmail)()

	setup := func(t *testing.T, sourceName, sub, userName, email string) (*auth_model.Source, *user_model.User) {
		t.Helper()
		srv := newFakeOIDCServer(t, FakeOIDCConfig{Sub: sub, OID: sub + "-oid", Email: email, Name: "OIDC Test User"})
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

// TestOAuth2CallbackReactivationGating exercises the gate in handleOAuth2SignIn:
// an inactive user can only be reactivated when who was disabled by auto-sync
func TestOAuth2CallbackReactivationGating(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&setting.OAuth2Client.EnableAutoRegistration, true)()
	defer test.MockVariableValue(&setting.OAuth2Client.Username, setting.OAuth2UsernameUserid)()

	srv := newFakeOIDCServer(t, FakeOIDCConfig{Sub: "test-sub", Email: "test@example.com", Name: "Test User"})
	addOAuth2Source(t, "test-oauth-source", oauth2.Source{
		Provider:                      "openidConnect",
		ClientID:                      "test-client-id",
		ClientSecret:                  "test-client-secret",
		OpenIDConnectAutoDiscoveryURL: srv.URL + "/.well-known/openid-configuration",
	})
	authSource, err := auth_model.GetActiveOAuth2SourceByAuthName(t.Context(), "test-oauth-source")
	require.NoError(t, err)

	u := &user_model.User{Name: "test-user", Email: "test@example.com"}
	require.NoError(t, user_model.CreateUser(t.Context(), u, &user_model.Meta{}))

	extLink := &user_model.ExternalLoginUser{
		UserID:        u.ID,
		LoginSourceID: authSource.ID,
		Provider:      authSource.Name,
		ExternalID:    "test-sub",
	}
	require.NoError(t, user_model.LinkExternalToUser(t.Context(), u, extLink))

	prepareUserExternalLink := func(t *testing.T, refreshToken string) {
		err := user_model.UpdateUserCols(t.Context(), &user_model.User{ID: u.ID, IsActive: false}, "is_active")
		require.NoError(t, err)
		_, err = db.GetEngine(t.Context()).Where(builder.Eq{"user_id": u.ID}).Cols("refresh_token").
			Update(&user_model.ExternalLoginUser{RefreshToken: refreshToken})
		require.NoError(t, err)
		require.False(t, unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: u.ID}).IsActive)
	}

	t.Run("admin-disabled user is not reactivated", func(t *testing.T) {
		prepareUserExternalLink(t, "non-empty-refresh-token")
		doOIDCSignIn(t, authSource.Name)
		after := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: u.ID})
		assert.False(t, after.IsActive, "OAuth callback must not re-enable an administrator-disabled account")
	})

	t.Run("auto-sync-disabled user is reactivated", func(t *testing.T) {
		prepareUserExternalLink(t, "" /* empty refresh token */)
		doOIDCSignIn(t, authSource.Name)
		after := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: u.ID})
		assert.True(t, after.IsActive, "OAuth callback must reactivate a sync-disabled account on successful login")
	})
}

// FakeOIDCConfig holds configuration for the fake OIDC server used in tests.
type FakeOIDCConfig struct {
	Sub    string
	OID    string
	Email  string
	Name   string
	Groups []string
}

// newFakeOIDCServer starts a httptest.Server that implements the minimum OIDC endpoints needed to complete a sign-in flow
func newFakeOIDCServer(t *testing.T, cfg FakeOIDCConfig) *httptest.Server {
	t.Helper()

	// Set defaults for backward compatibility with existing tests
	if cfg.Email == "" {
		cfg.Email = cfg.Sub + "@example.com"
	}
	if cfg.Name == "" {
		cfg.Name = "OIDC Test User"
	}

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
				"iss":   srv.URL,
				"aud":   "test-client-id",
				"exp":   time.Now().Add(time.Hour).Unix(),
				"sub":   cfg.Sub,
				"email": cfg.Email,
				"name":  cfg.Name,
			}
			if cfg.OID != "" {
				claims["oid"] = cfg.OID
			}
			if cfg.Groups != nil {
				claims["groups"] = cfg.Groups
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
			response := map[string]any{
				"sub":   cfg.Sub,
				"email": cfg.Email,
				"name":  cfg.Name,
			}
			if cfg.OID != "" {
				response["oid"] = cfg.OID
			}
			if cfg.Groups != nil {
				response["groups"] = cfg.Groups
			}
			_ = json.NewEncoder(w).Encode(response)
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

// newOIDCSource is a helper function to create a configured OAuth2 source for testing
func newOIDCSource(srv *httptest.Server, withAdmin, withRestricted bool) oauth2.Source {
	src := oauth2.Source{
		Provider:                      "openidConnect",
		ClientID:                      "test-client-id",
		ClientSecret:                  "test-client-secret",
		OpenIDConnectAutoDiscoveryURL: srv.URL + "/.well-known/openid-configuration",
		GroupClaimName:                "groups",
	}
	if withAdmin {
		src.AdminGroup = "admins"
	}
	if withRestricted {
		src.RestrictedGroup = "restricted-users"
	}
	return src
}

// TestOAuth2GroupClaimsAppliedOnFirstLogin verifies that group claims from OAuth2/OIDC
// are correctly applied to newly created users on the first login
func TestOAuth2GroupClaimsAppliedOnFirstLogin(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	// Enable auto-registration to ensure first login creates user with group claims
	defer test.MockVariableValue(&setting.OAuth2Client.EnableAutoRegistration, true)()
	// Use sub claim as username for deterministic user naming
	defer test.MockVariableValue(&setting.OAuth2Client.Username, setting.OAuth2UsernameUserid)()

	tt := []struct {
		Name         string
		IsAdmin      bool
		IsRestricted bool
		SourceName   string
	}{
		{
			Name:         "user in both admin and restricted groups",
			IsAdmin:      true,
			IsRestricted: true,
			SourceName:   "test-group-claims",
		},
		{
			Name:         "no groups",
			IsAdmin:      false,
			IsRestricted: false,
			SourceName:   "test-no-groups",
		},
	}
	for _, tc := range tt {
		t.Run(tc.Name, func(t *testing.T) {
			// Set up OIDC server with group claims
			srv := newFakeOIDCServer(t, FakeOIDCConfig{
				Sub:    tc.SourceName,
				Email:  tc.SourceName + "@example.com",
				Name:   "Test User",
				Groups: []string{"admins", "restricted-users"},
			})

			// Ensure it's the first login so no user in database
			unittest.AssertNotExistsBean(t, &user_model.User{Name: tc.SourceName})

			addOAuth2Source(t, tc.SourceName, newOIDCSource(srv, tc.IsAdmin, tc.IsRestricted))

			doOIDCSignIn(t, tc.SourceName)

			user := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: tc.SourceName})
			assert.Equal(t, tc.IsAdmin, user.IsAdmin)
			assert.Equal(t, tc.IsRestricted, user.IsRestricted)
			assert.Equal(t, auth_model.OAuth2, user.LoginType)
		})
	}
}

// TestOAuth2GroupClaimsManualLinking tests that group claims are applied correctly
// when a user goes through the manual linking flow (auto-registration disabled).
func TestOAuth2GroupClaimsManualLinking(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	// Disable auto-registration to force manual linking flow
	defer test.MockVariableValue(&setting.OAuth2Client.EnableAutoRegistration, false)()
	defer test.MockVariableValue(&setting.Service.AllowOnlyInternalRegistration, false)()

	tt := []struct {
		Name         string
		IsAdmin      bool
		IsRestricted bool
		SourceName   string
	}{
		{
			Name:         "user in both admin and restricted groups",
			IsAdmin:      true,
			IsRestricted: true,
			SourceName:   "test-group-claims-manual-linking",
		},
		{
			Name:         "no groups",
			IsAdmin:      false,
			IsRestricted: false,
			SourceName:   "test-no-groups-manual-linking",
		},
	}

	for _, tc := range tt {
		t.Run(tc.Name, func(t *testing.T) {
			srv := newFakeOIDCServer(t, FakeOIDCConfig{
				Sub:    tc.SourceName,
				Email:  tc.SourceName + "@example.com",
				Name:   "Manual User",
				Groups: []string{"admins", "restricted-users"},
			})
			addOAuth2Source(t, tc.SourceName, newOIDCSource(srv, tc.IsAdmin, tc.IsRestricted))
			unittest.AssertNotExistsBean(t, &user_model.User{Name: tc.SourceName})
			session := emptyTestSession(t)
			resp := session.MakeRequest(t, NewRequest(t, "GET", "/user/oauth2/"+tc.SourceName), http.StatusTemporaryRedirect)

			location := resp.Header().Get("Location")
			u, err := url.Parse(location)
			require.NoError(t, err)
			state := u.Query().Get("state")
			require.NotEmpty(t, state, "redirect to OIDC provider must include state")

			callbackURL := fmt.Sprintf("/user/oauth2/%s/callback?code=test-code&state=%s", tc.SourceName, url.QueryEscape(state))
			session.MakeRequest(t, NewRequest(t, "GET", callbackURL), http.StatusSeeOther)

			// Submit the form to create a new account
			linkAccountResp := session.MakeRequest(t, NewRequest(t, "GET", "/user/link_account"), http.StatusOK)
			// Verify we're on the link account page
			assert.Contains(t, linkAccountResp.Body.String(), "link_account")

			// Use NewRequestWithValues to POST form data (no CSRF needed in tests)
			// Field names are lowercase in HTML forms: user_name, email, password, retype
			req := NewRequestWithValues(t, "POST", "/user/link_account_signup", map[string]string{
				"user_name": tc.SourceName,
				"email":     tc.SourceName + "@example.com",
				"password":  "", // AllowOnlyExternalRegistration means no password needed
				"retype":    "",
			})
			session.MakeRequest(t, req, http.StatusSeeOther)

			user := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: tc.SourceName})
			assert.Equal(t, tc.IsAdmin, user.IsAdmin)
			assert.Equal(t, tc.IsRestricted, user.IsRestricted)
			assert.Equal(t, auth_model.OAuth2, user.LoginType)
		})
	}
}
