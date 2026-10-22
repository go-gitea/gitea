// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"encoding/base64"
	"net/http"
	"strings"
	"testing"

	"gitea.dev/models/auth"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/auth/httpauth"
	"gitea.dev/modules/json"
	"gitea.dev/modules/test"
	"gitea.dev/modules/web"
	"gitea.dev/services/contexttest"
	"gitea.dev/services/forms"
	"gitea.dev/services/oauth2_provider"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOAuth2FormEncodedBasicAuth(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	signingKey, err := oauth2_provider.CreateJWTSigningKey("HS256", make([]byte, 32))
	require.NoError(t, err)
	defer test.MockVariableValue(&oauth2_provider.DefaultSigningKey, signingKey)()

	app := unittest.AssertExistsAndLoadBean(t, &auth.OAuth2Application{ID: 1})
	const clientSecret = "4MK8Na6R55smdCY0WuCCumZ6hjRPnGY5saWVRHHjJiA=" // plaintext of the fixture's client_secret
	// RFC 6749 section 2.3.1: clients form-urlencode both values before base64, some even escape "-"
	escape := strings.NewReplacer("-", "%2D", "=", "%3D").Replace
	basicAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(escape(app.ClientID)+":"+escape(clientSecret)))

	for _, tt := range []struct{ name, bodyClientID string }{
		{"header only", ""},
		{"body client_id equals decoded header", app.ClientID},
	} {
		t.Run("AccessToken/"+tt.name, func(t *testing.T) {
			ctx, resp := contexttest.MockContext(t, "POST /login/oauth/access_token")
			ctx.Req.Header.Set("Authorization", basicAuth)
			web.SetForm(ctx, &forms.AccessTokenForm{GrantType: "authorization_code", ClientID: tt.bodyClientID})
			AccessTokenOAuth(ctx)

			var accessTokenErr oauth2_provider.AccessTokenError
			require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &accessTokenErr))
			assert.Equal(t, "client is not authorized", accessTokenErr.ErrorDescription) // the client was accepted, only the code is unknown
		})
	}

	t.Run("Introspect", func(t *testing.T) {
		ctx, resp := contexttest.MockContext(t, "POST /login/oauth/introspect")
		ctx.Req.Header.Set("Authorization", basicAuth)
		web.SetForm(ctx, &forms.IntrospectTokenForm{Token: "invalid"})
		IntrospectOAuth(ctx)
		assert.Equal(t, http.StatusOK, resp.Code) // inactive token, not 401 for rejected client credentials
	})
}

func TestBasicAuthClientCredentials(t *testing.T) {
	for _, tt := range []struct{ name, username, password, wantID, wantSecret string }{
		{"percent-encoded", "a%2Db", "s%3D%2B%2F", "a-b", "s=+/"},
		{"plus is a space", "a+b", "c+d", "a b", "c d"},
		{"invalid escape stays raw per value", "a%2Db", "c%zz", "a-b", "c%zz"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			clientID, clientSecret := basicAuthClientCredentials(&httpauth.BasicAuth{Username: tt.username, Password: tt.password})
			assert.Equal(t, tt.wantID, clientID)
			assert.Equal(t, tt.wantSecret, clientSecret)
		})
	}
}
