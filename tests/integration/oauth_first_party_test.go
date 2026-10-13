// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/unittest"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An application flagged SkipSecondaryAuthorization is the first-party client
// that *is* the site being signed in to. It is never shown a consent screen --
// not on the first authorization, and not when the scope it asks for changes --
// and its grant is kept in step without one.
//
// What is checked here is the grant that gets left behind, because the token's
// capabilities are read from the grant and not from the request, so a grant
// that quietly loses a scope is a sign-in that quietly stops working.
func TestOAuth2FirstPartyGrant(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	app := createFirstPartyTestApplication(t, "user2", "first-party")

	authorize := func(scope string) *http.Response {
		query := url.Values{
			"client_id":     {app.ClientID},
			"redirect_uri":  {"https://example.com/"},
			"response_type": {"code"},
			"state":         {"thestate"},
		}
		if scope != "" {
			query.Set("scope", scope)
		}
		req := NewRequest(t, "GET", "/login/oauth/authorize?"+query.Encode())
		return loginUser(t, "user4").MakeRequest(t, req, http.StatusSeeOther).Result()
	}

	grantScope := func() string {
		grant := unittest.AssertExistsAndLoadBean(t, &auth_model.OAuth2Grant{ApplicationID: app.ID})
		return grant.Scope
	}

	t.Run("FirstAuthorizationAsksNothing", func(t *testing.T) {
		// No grant exists yet. Stock Gitea renders the consent screen here --
		// 200, not 303 -- and only creates the grant once somebody clicks.
		resp := authorize("openid profile")
		location, err := resp.Location()
		require.NoError(t, err)
		assert.Equal(t, "thestate", location.Query().Get("state"))
		assert.NotEmpty(t, location.Query().Get("code"))
		assert.Equal(t, "openid profile", grantScope())
	})

	t.Run("AWiderScopeIsTakenWithoutAsking", func(t *testing.T) {
		// Otherwise a consent screen appears once per release that touches
		// scopes, and before the widening existed this was an outright
		// "a grant exists with different scope" server error.
		authorize("openid profile email")
		assert.Equal(t, "openid profile email", grantScope())
	})

	t.Run("ANarrowerScopeDoesNotTakeAnythingAway", func(t *testing.T) {
		// Replacement rather than union would strip `email` from an
		// authorization the user had already given, for every token issued
		// afterwards.
		authorize("openid")
		assert.Equal(t, "openid profile email", grantScope())
	})

	t.Run("NoScopeAtAllDoesNotEmptyTheGrant", func(t *testing.T) {
		// The scope parameter is optional (RFC 6749 Section 3.3). Replacement
		// emptied the grant outright here, taking `openid` with it, so the
		// next sign-in returned no ID token at all.
		authorize("")
		assert.Equal(t, "openid profile email", grantScope())
	})
}

func createFirstPartyTestApplication(t *testing.T, userName, name string) *api.OAuth2Application {
	t.Helper()
	req := NewRequestWithJSON(t, "POST", "/api/v1/user/applications/oauth2", &api.CreateOAuth2ApplicationOptions{
		Name:                       name,
		RedirectURIs:               []string{"https://example.com/"},
		ConfidentialClient:         true,
		SkipSecondaryAuthorization: true,
	}).AddBasicAuth(userName)
	resp := MakeRequest(t, req, http.StatusCreated)
	created := DecodeJSON(t, resp, &api.OAuth2Application{})
	require.NotEmpty(t, created.ClientID)
	return created
}
