// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
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
	const fixtureClientSecretPlaintext = "4MK8Na6R55smdCY0WuCCumZ6hjRPnGY5saWVRHHjJiA="
	escape := strings.NewReplacer("-", "%2D", "=", "%3D").Replace

	t.Run("AccessToken accepts client matching body client_id and rejects unknown code", func(t *testing.T) {
		ctx, resp := contexttest.MockContext(t, "POST /login/oauth/access_token")
		ctx.Req.SetBasicAuth(escape(app.ClientID), escape(fixtureClientSecretPlaintext))
		web.SetForm(ctx, &forms.AccessTokenForm{GrantType: "authorization_code", ClientID: app.ClientID})
		AccessTokenOAuth(ctx)

		var accessTokenErr oauth2_provider.AccessTokenError
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &accessTokenErr))
		assert.Equal(t, "client is not authorized", accessTokenErr.ErrorDescription)
	})

	t.Run("Introspect accepts client and reports inactive token", func(t *testing.T) {
		ctx, resp := contexttest.MockContext(t, "POST /login/oauth/introspect")
		ctx.Req.SetBasicAuth(escape(app.ClientID), escape(fixtureClientSecretPlaintext))
		web.SetForm(ctx, &forms.IntrospectTokenForm{Token: "invalid"})
		IntrospectOAuth(ctx)
		assert.Equal(t, http.StatusOK, resp.Code)
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
