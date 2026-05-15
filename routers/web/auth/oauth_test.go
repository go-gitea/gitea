// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"net"
	"net/http"
	"net/url"
	"testing"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/hostmatcher"
	"code.gitea.io/gitea/services/auth/source/oauth2"
	"code.gitea.io/gitea/services/oauth2_provider"

	"github.com/golang-jwt/jwt/v5"
	"github.com/markbates/goth"
	goth_oidc "github.com/markbates/goth/providers/openidConnect"
	"github.com/stretchr/testify/assert"
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

func TestOauth2AvatarAllowList(t *testing.T) {
	allowList := oauth2AvatarAllowList("", nil)
	assert.True(t, allowList.MatchIPAddr(net.ParseIP("8.8.8.8")))
	assert.False(t, allowList.MatchIPAddr(net.ParseIP("127.0.0.1")))

	allowList = oauth2AvatarAllowList("", &oauth2.Source{Provider: "github"})
	assert.True(t, allowList.MatchHostName("github.com"))
	assert.True(t, allowList.MatchHostName("api.github.com"))

	source := &oauth2.Source{
		OpenIDConnectAutoDiscoveryURL: "https://idp.internal/.well-known/openid-configuration",
		CustomURLMapping: &oauth2.CustomURLMapping{
			AuthURL:    "https://login.example.test/oauth2/auth",
			TokenURL:   "https://login.example.test/oauth2/token",
			ProfileURL: "https://profile.example.test/userinfo",
			EmailURL:   "https://mail.example.test/me",
		},
	}
	allowList = oauth2AvatarAllowList("", source)
	assert.True(t, allowList.MatchHostName("idp.internal"))
	assert.True(t, allowList.MatchHostName("login.example.test"))
	assert.True(t, allowList.MatchHostName("profile.example.test"))
	assert.True(t, allowList.MatchHostName("mail.example.test"))
	assert.False(t, allowList.MatchHostName("unexpected.example.test"))

	oidcProvider, err := goth_oidc.NewCustomisedURL(
		"client",
		"secret",
		"https://gitea.example/user/oauth2/test-oidc-provider/callback",
		"https://login.oidc.internal/auth",
		"https://login.oidc.internal/token",
		"https://issuer.oidc.internal",
		"https://userinfo.oidc.internal",
		"",
		"openid",
	)
	assert.NoError(t, err)
	oidcProvider.SetName("test-oidc-provider")
	goth.UseProviders(oidcProvider)
	t.Cleanup(func() {
		oauth2.RemoveProviderFromGothic("test-oidc-provider")
	})

	allowList = oauth2AvatarAllowList("test-oidc-provider", &oauth2.Source{
		Provider:                      "openidConnect",
		OpenIDConnectAutoDiscoveryURL: "https://discovery.oidc.internal/.well-known/openid-configuration",
	})
	assert.True(t, allowList.MatchHostName("discovery.oidc.internal"))
	assert.True(t, allowList.MatchHostName("login.oidc.internal"))
	assert.True(t, allowList.MatchHostName("userinfo.oidc.internal"))
	assert.True(t, allowList.MatchHostName("issuer.oidc.internal"))
}

func TestOauth2AvatarProxyChecksTargetURL(t *testing.T) {
	allowList := oauth2AvatarAllowList("", nil)
	blockList := hostmatcher.ParseHostMatchList("oauth2 avatar host", "")
	proxyURL, err := url.Parse("http://127.0.0.1:3128")
	assert.NoError(t, err)

	proxyFunc := oauth2AvatarProxy(allowList, blockList, func(req *http.Request) (*url.URL, error) {
		return proxyURL, nil
	})

	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1/avatar.png", nil)
	assert.NoError(t, err)
	gotProxyURL, err := proxyFunc(req)
	assert.Nil(t, gotProxyURL)
	assert.Error(t, err)

	req, err = http.NewRequest(http.MethodGet, "http://8.8.8.8/avatar.png", nil)
	assert.NoError(t, err)
	gotProxyURL, err = proxyFunc(req)
	assert.NoError(t, err)
	assert.Equal(t, proxyURL, gotProxyURL)
}
