// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"gitea.dev/models/auth"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/hostmatcher"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/services/oauth2_provider"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createAndParseToken(t *testing.T, grant *auth.OAuth2Grant) *oauth2_provider.OIDCToken {
	signingKey, err := oauth2_provider.CreateJWTSigningKey("HS256", make([]byte, 32))
	assert.NoError(t, err)
	assert.NotNil(t, signingKey)

	response, terr := oauth2_provider.NewAccessTokenResponse(t.Context(), grant, signingKey, signingKey)
	assert.Nil(t, terr)
	assert.NotNil(t, response)

	parsedToken, err := jwt.ParseWithClaims(response.IDToken, &oauth2_provider.OIDCToken{}, func(token *jwt.Token) (any, error) {
		assert.NotNil(t, token.Method)
		assert.Equal(t, signingKey.SigningMethod().Alg(), token.Method.Alg())
		return signingKey.VerifyKey(), nil
	})
	assert.NoError(t, err)
	assert.True(t, parsedToken.Valid)

	oidcToken, ok := parsedToken.Claims.(*oauth2_provider.OIDCToken)
	assert.True(t, ok)
	assert.NotNil(t, oidcToken)

	return oidcToken
}

func TestNewAccessTokenResponse_OIDCToken(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	grants, err := auth.GetOAuth2GrantsByUserID(t.Context(), 3)
	assert.NoError(t, err)
	assert.Len(t, grants, 1)

	// Scopes: openid
	oidcToken := createAndParseToken(t, grants[0])
	assert.Empty(t, oidcToken.Name)
	assert.Empty(t, oidcToken.PreferredUsername)
	assert.Empty(t, oidcToken.Profile)
	assert.Empty(t, oidcToken.Picture)
	assert.Empty(t, oidcToken.Website)
	assert.Empty(t, oidcToken.UpdatedAt)
	assert.Empty(t, oidcToken.Email)
	assert.False(t, oidcToken.EmailVerified)

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	grants, err = auth.GetOAuth2GrantsByUserID(t.Context(), user.ID)
	assert.NoError(t, err)
	assert.Len(t, grants, 1)

	// Scopes: openid profile email
	oidcToken = createAndParseToken(t, grants[0])
	assert.Equal(t, user.DisplayName(), oidcToken.Name)
	assert.Equal(t, user.Name, oidcToken.PreferredUsername)
	assert.Equal(t, user.HTMLURL(t.Context()), oidcToken.Profile)
	assert.Equal(t, user.AvatarLink(t.Context()), oidcToken.Picture)
	assert.Equal(t, user.Website, oidcToken.Website)
	assert.Equal(t, user.UpdatedUnix, oidcToken.UpdatedAt)
	assert.Equal(t, user.Email, oidcToken.Email)
	assert.Equal(t, user.IsActive, oidcToken.EmailVerified)
}

func TestOAuth2AvatarClientBlocksLoopback(t *testing.T) {
	var hit atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit.Store(true)
		_, _ = w.Write([]byte("img"))
	}))
	defer srv.Close()

	// the httptest server binds a loopback address, which the SSRF-protected dialer must refuse
	resp, err := oauth2AvatarHTTPClient().Get(srv.URL)
	if resp != nil {
		_ = resp.Body.Close()
	}
	require.Error(t, err)
	assert.False(t, hit.Load(), "avatar client must refuse to dial a loopback address")
}

func TestOAuth2AvatarAllowListRestricts(t *testing.T) {
	defer test.MockVariableValue(&setting.Security.AllowedHostList)()

	// a configured allow-list must actually narrow reachable hosts below "all external"
	setting.Security.AllowedHostList = "avatars.example.com"
	allowList := oauth2AvatarAllowList()
	assert.True(t, allowList.MatchHostName("avatars.example.com"), "the configured host must be allowed")
	assert.False(t, allowList.MatchHostName("8.8.8.8"), "an unrelated external host must be rejected")

	// the default `external` allow-list still permits external hosts
	setting.Security.AllowedHostList = hostmatcher.MatchBuiltinExternal
	assert.True(t, oauth2AvatarAllowList().MatchHostName("8.8.8.8"), "default allow-list permits external hosts")
}

func TestOAuth2AvatarClientBlocksCloudMetadata(t *testing.T) {
	// external-only allow-list must reject link-local cloud metadata (169.254.169.254) at dial time
	resp, err := oauth2AvatarHTTPClient().Get("http://169.254.169.254/latest/meta-data/")
	if resp != nil {
		_ = resp.Body.Close()
	}
	require.Error(t, err)
	assert.ErrorContains(t, err, "can only call allowed HTTP servers",
		"avatar client must refuse a link-local cloud-metadata address")
}
