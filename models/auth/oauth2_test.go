// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth_test

import (
	"testing"
	"time"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
)

func TestOAuth2AuthorizationCode(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	t.Run("GenerateSetsValidUntil", func(t *testing.T) {
		grant := unittest.AssertExistsAndLoadBean(t, &auth_model.OAuth2Grant{ID: 1})
		expectedValidUntil := timeutil.TimeStamp(time.Now().Unix() + 600)
		code, err := grant.GenerateNewAuthorizationCode(t.Context(), "http://127.0.0.1/", "", "")
		require.NoError(t, err)
		assert.Equal(t, expectedValidUntil, code.ValidUntil)
		assert.False(t, code.IsExpired())
		assert.Equal(t, int64(1), code.ID)

		code2, err := auth_model.GetOAuth2AuthorizationByCode(t.Context(), code.Code)
		require.NoError(t, err)
		assert.Equal(t, code.Code, code2.Code)

		assert.NoError(t, code.Invalidate(t.Context()))

		code, err = auth_model.GetOAuth2AuthorizationByCode(t.Context(), "does not exist")
		require.NoError(t, err)
		require.Nil(t, code)
	})

	t.Run("Expired", func(t *testing.T) {
		defer timeutil.MockSet(time.Unix(2, 0).UTC())()

		code := &auth_model.OAuth2AuthorizationCode{ValidUntil: timeutil.TimeStamp(1)}
		assert.True(t, code.IsExpired())
	})

	t.Run("Invalidate", func(t *testing.T) {
		grant := unittest.AssertExistsAndLoadBean(t, &auth_model.OAuth2Grant{ID: 1})
		code, err := grant.GenerateNewAuthorizationCode(t.Context(), "http://127.0.0.1/", "", "")
		require.NoError(t, err)
		require.NotNil(t, code)
		require.NoError(t, code.Invalidate(t.Context()))
		unittest.AssertNotExistsBean(t, &auth_model.OAuth2AuthorizationCode{Code: code.Code})
		assert.ErrorIs(t, code.Invalidate(t.Context()), auth_model.ErrOAuth2AuthorizationCodeInvalidated)
	})
}

func TestOAuth2ApplicationAndGrant(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	t.Run("GetOAuth2ApplicationByClientID", func(t *testing.T) {
		app, err := auth_model.GetOAuth2ApplicationByClientID(t.Context(), "da7da3ba-9a13-4167-856f-3899de0b0138")
		assert.NoError(t, err)
		assert.Equal(t, "da7da3ba-9a13-4167-856f-3899de0b0138", app.ClientID)

		app, err = auth_model.GetOAuth2ApplicationByClientID(t.Context(), "invalid client id")
		assert.Error(t, err)
		assert.Nil(t, app)
	})

	t.Run("OAuth2Application_GetGrantByUserID", func(t *testing.T) {
		app := unittest.AssertExistsAndLoadBean(t, &auth_model.OAuth2Application{ID: 1})
		grant, err := app.GetGrantByUserID(t.Context(), 1)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), grant.UserID)

		grant, err = app.GetGrantByUserID(t.Context(), 34923458)
		assert.NoError(t, err)
		assert.Nil(t, grant)
	})

	t.Run("GetOAuth2GrantByID", func(t *testing.T) {
		grant, err := auth_model.GetOAuth2GrantByID(t.Context(), 1)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), grant.ID)

		grant, err = auth_model.GetOAuth2GrantByID(t.Context(), 34923458)
		assert.NoError(t, err)
		assert.Nil(t, grant)
	})

	t.Run("OAuth2Grant_ScopeContains", func(t *testing.T) {
		grant := unittest.AssertExistsAndLoadBean(t, &auth_model.OAuth2Grant{ID: 1, Scope: "openid profile"})
		assert.True(t, grant.ScopeContains("openid"))
		assert.True(t, grant.ScopeContains("profile"))
		assert.False(t, grant.ScopeContains("profil"))
		assert.False(t, grant.ScopeContains("profile2"))
	})

	t.Run("GetOAuth2GrantsByUserID", func(t *testing.T) {
		result, err := auth_model.GetOAuth2GrantsByUserID(t.Context(), 1)
		assert.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, int64(1), result[0].ID)
		assert.Equal(t, result[0].ApplicationID, result[0].Application.ID)

		result, err = auth_model.GetOAuth2GrantsByUserID(t.Context(), 34134)
		assert.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("OAuth2Grant_GenerateNewAuthorizationCode", func(t *testing.T) {
		grant := unittest.AssertExistsAndLoadBean(t, &auth_model.OAuth2Grant{ID: 1})
		code, err := grant.GenerateNewAuthorizationCode(t.Context(), "https://example2.com/callback", "CjvyTLSdR47G5zYenDA-eDWW4lRrO8yvjcWwbD_deOg", "S256")
		assert.NoError(t, err)
		assert.NotNil(t, code)
		assert.Greater(t, len(code.Code), 32) // secret length > 32
	})

	t.Run("OAuth2Application_CreateGrant", func(t *testing.T) {
		app := unittest.AssertExistsAndLoadBean(t, &auth_model.OAuth2Application{ID: 1})
		grant, err := app.CreateGrant(t.Context(), 2, "")
		assert.NoError(t, err)
		assert.NotNil(t, grant)
		assert.Equal(t, int64(2), grant.UserID)
		assert.Equal(t, int64(1), grant.ApplicationID)
		assert.Empty(t, grant.Scope)
	})

	t.Run("OAuth2Grant_IncreaseCounter", func(t *testing.T) {
		grant := unittest.AssertExistsAndLoadBean(t, &auth_model.OAuth2Grant{ID: 1, Counter: 1})
		stale := *grant
		assert.NoError(t, grant.IncreaseCounter(t.Context()))
		assert.Equal(t, int64(2), grant.Counter)
		unittest.AssertExistsAndLoadBean(t, &auth_model.OAuth2Grant{ID: 1, Counter: 2})
		err := stale.IncreaseCounter(t.Context())
		assert.ErrorIs(t, err, auth_model.ErrOAuth2GrantStaleCounter)
	})

	t.Run("OAuth2Application_GenerateClientSecret", func(t *testing.T) {
		app := unittest.AssertExistsAndLoadBean(t, &auth_model.OAuth2Application{ID: 1})
		secret, err := app.GenerateClientSecret(t.Context())
		assert.NoError(t, err)
		assert.NotEmpty(t, secret)
		unittest.AssertExistsAndLoadBean(t, &auth_model.OAuth2Application{ID: 1, ClientSecret: app.ClientSecret})
		assert.True(t, app.ValidateClientSecret([]byte(secret)))
		assert.False(t, app.ValidateClientSecret([]byte("fewijfowejgfiowjeoifew")))

		defer test.MockVariableValue(&setting.IsInTesting, false)()
		_, err = app.GenerateClientSecret(t.Context())
		require.NoError(t, err)
		cost, err := bcrypt.Cost([]byte(app.ClientSecret))
		require.NoError(t, err)
		assert.Equal(t, bcrypt.DefaultCost, cost)
	})

	t.Run("CreateOAuth2Application", func(t *testing.T) {
		app, err := auth_model.CreateOAuth2Application(t.Context(), auth_model.CreateOAuth2ApplicationOptions{Name: "newapp", UserID: 1})
		assert.NoError(t, err)
		assert.Equal(t, "newapp", app.Name)
		assert.Len(t, app.ClientID, 36)
		unittest.AssertExistsAndLoadBean(t, &auth_model.OAuth2Application{Name: "newapp"})
	})
}

func BenchmarkOAuth2Application_GenerateClientSecret(b *testing.B) {
	assert.NoError(b, unittest.PrepareTestDatabase())
	app := unittest.AssertExistsAndLoadBean(b, &auth_model.OAuth2Application{ID: 1})
	defer test.MockVariableValue(&setting.IsInTesting, false)()
	for b.Loop() {
		_, _ = app.GenerateClientSecret(b.Context())
	}
}

func TestOAuth2Application_ContainsRedirectURI(t *testing.T) {
	app := &auth_model.OAuth2Application{
		RedirectURIs: []string{"a", "b", "c"},
	}
	assert.True(t, app.ContainsRedirectURI("a"))
	assert.True(t, app.ContainsRedirectURI("b"))
	assert.True(t, app.ContainsRedirectURI("c"))
	assert.False(t, app.ContainsRedirectURI("d"))
}

func TestOAuth2Application_ContainsRedirectURI_WithPort(t *testing.T) {
	app := &auth_model.OAuth2Application{
		RedirectURIs:       []string{"http://127.0.0.1/", "http://::1/", "http://192.168.0.1/", "http://intranet/", "https://127.0.0.1/"},
		ConfidentialClient: false,
	}

	// http loopback uris should ignore port
	// https://datatracker.ietf.org/doc/html/rfc8252#section-7.3
	assert.True(t, app.ContainsRedirectURI("http://127.0.0.1:3456/"))
	assert.True(t, app.ContainsRedirectURI("http://127.0.0.1/"))
	assert.True(t, app.ContainsRedirectURI("http://[::1]:3456/"))

	// not http
	assert.False(t, app.ContainsRedirectURI("https://127.0.0.1:3456/"))
	// not loopback
	assert.False(t, app.ContainsRedirectURI("http://192.168.0.1:9954/"))
	assert.False(t, app.ContainsRedirectURI("http://intranet:3456/"))
	// unparseable
	assert.False(t, app.ContainsRedirectURI(":"))
}

func TestOAuth2Application_ContainsRedirect_Slash(t *testing.T) {
	for _, redirectURI := range []string{"http://127.0.0.1", "http://127.0.0.1/"} {
		app := &auth_model.OAuth2Application{RedirectURIs: []string{redirectURI}}
		assert.True(t, app.ContainsRedirectURI("http://127.0.0.1"))
		assert.True(t, app.ContainsRedirectURI("http://127.0.0.1/"))
		assert.False(t, app.ContainsRedirectURI("http://127.0.0.1/other"))
	}
}

func TestOAuth2Application_ContainsRedirectURI_ASCIIOnlyNormalization(t *testing.T) {
	testCases := []struct {
		name        string
		registered  []string
		redirectURI string
		allowed     bool
	}{
		{
			name:        "exact-match",
			registered:  []string{"https://signin.example.test/callback"},
			redirectURI: "https://signin.example.test/callback",
			allowed:     true,
		},
		{
			name:        "ascii-case-insensitive",
			registered:  []string{"https://signin.example.test/callback"},
			redirectURI: "https://signIN.example.test/callback",
			allowed:     true,
		},
		{
			name:        "non-ascii-not-folded",
			registered:  []string{"https://signin.example.test/callback"},
			redirectURI: "https://signİn.example.test/callback",
			allowed:     false,
		},
		{
			name:        "loopback-strips-port",
			registered:  []string{"http://127.0.0.1/callback"},
			redirectURI: "http://127.0.0.1:12345/callback",
			allowed:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			app := &auth_model.OAuth2Application{RedirectURIs: tc.registered}
			assert.Equal(t, tc.allowed, app.ContainsRedirectURI(tc.redirectURI))
		})
	}
}

func TestOAuth2Application_TableName(t *testing.T) {
	assert.Equal(t, "oauth2_application", new(auth_model.OAuth2Application).TableName())
}

//////////////////// Grant

func TestOAuth2Grant_TableName(t *testing.T) {
	assert.Equal(t, "oauth2_grant", new(auth_model.OAuth2Grant).TableName())
}

func TestRevokeOAuth2Grant(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	assert.NoError(t, auth_model.RevokeOAuth2Grant(t.Context(), 1, 1))
	unittest.AssertNotExistsBean(t, &auth_model.OAuth2Grant{ID: 1, UserID: 1})
}

//////////////////// Authorization Code

func TestOAuth2AuthorizationCode_ValidateCodeChallenge(t *testing.T) {
	s256Verifier := "s256-verifier"
	s256Challenge := oauth2.S256ChallengeFromVerifier(s256Verifier)
	missingVerifierChallenge := oauth2.S256ChallengeFromVerifier("verifier-not-supplied")

	testCases := []struct {
		name      string
		method    string
		challenge string
		verifier  string
		valid     bool
	}{
		{"plain-success", "plain", "plain-secret", "plain-secret", true},
		{"plain-failure", "plain", "plain-secret", "ierwgjoergjio", false},
		{"s256-success", "S256", s256Challenge, s256Verifier, true},
		{"s256-failure", "S256", s256Challenge, "wiogjerogorewngoenrgoiuenorg", false},
		{"unsupported-method", "monkey", "foiwgjioriogeiogjerger", "foiwgjioriogeiogjerger", false},
		{"no-pkce-configured", "", "", "", true},
		{"s256-missing-verifier", "S256", missingVerifierChallenge, "", false},
		{"plain-missing-verifier", "plain", "plain-secret", "", false},
		{"missing-method-with-challenge", "", "foierjiogerogerg", "", false},
		{"missing-method-rejects-even-matching-verifier", "", "foierjiogerogerg", "foierjiogerogerg", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			code := &auth_model.OAuth2AuthorizationCode{
				CodeChallengeMethod: tc.method,
				CodeChallenge:       tc.challenge,
			}
			assert.Equal(t, tc.valid, code.ValidateCodeChallenge(tc.verifier))
		})
	}
}

func TestOAuth2AuthorizationCode_GenerateRedirectURI(t *testing.T) {
	code := &auth_model.OAuth2AuthorizationCode{
		RedirectURI: "https://example.com/callback",
		Code:        "thecode",
	}

	redirect, err := code.GenerateRedirectURI("thestate")
	assert.NoError(t, err)
	assert.Equal(t, "https://example.com/callback?code=thecode&state=thestate", redirect.String())

	redirect, err = code.GenerateRedirectURI("")
	assert.NoError(t, err)
	assert.Equal(t, "https://example.com/callback?code=thecode", redirect.String())
}

func TestOAuth2AuthorizationCode_TableName(t *testing.T) {
	assert.Equal(t, "oauth2_authorization_code", new(auth_model.OAuth2AuthorizationCode).TableName())
}
