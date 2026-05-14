// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2

import (
	"testing"
	"time"

	"github.com/markbates/goth"
	goth_oidc "github.com/markbates/goth/providers/openidConnect"
	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"
)

type fakeProvider struct{}

func (p *fakeProvider) Name() string {
	return "fake"
}

func (p *fakeProvider) SetName(name string) {}

func (p *fakeProvider) BeginAuth(state string) (goth.Session, error) {
	return nil, nil //nolint:nilnil // the auth method is not applicable
}

func (p *fakeProvider) UnmarshalSession(string) (goth.Session, error) {
	return nil, nil //nolint:nilnil // the auth method is not applicable
}

func (p *fakeProvider) FetchUser(goth.Session) (goth.User, error) {
	return goth.User{}, nil
}

func (p *fakeProvider) Debug(bool) {
}

func (p *fakeProvider) RefreshToken(refreshToken string) (*oauth2.Token, error) {
	switch refreshToken {
	case "expired":
		return nil, &oauth2.RetrieveError{
			ErrorCode: "invalid_grant",
		}
	default:
		return &oauth2.Token{
			AccessToken:  "token",
			TokenType:    "Bearer",
			RefreshToken: "refresh",
			Expiry:       time.Now().Add(time.Hour),
		}, nil
	}
}

func (p *fakeProvider) RefreshTokenAvailable() bool {
	return true
}

func init() {
	RegisterGothProvider(
		NewSimpleProvider("fake", "Fake", []string{"account"},
			func(clientKey, secret, callbackURL string, scopes ...string) goth.Provider {
				return &fakeProvider{}
			}))
}

func TestGetSourceEndpointURLs(t *testing.T) {
	t.Run("custom provider defaults and overrides", func(t *testing.T) {
		urls := GetSourceEndpointURLs("", &Source{
			Provider: "github",
			CustomURLMapping: &CustomURLMapping{
				AuthURL: "https://login.github.example.test/oauth/authorize",
			},
		})

		assert.ElementsMatch(t, []string{
			"https://login.github.example.test/oauth/authorize",
			"https://github.com/login/oauth/access_token",
			"https://api.github.com/user",
			"https://api.github.com/user/emails",
		}, urls)
	})

	t.Run("registered oidc provider", func(t *testing.T) {
		provider, err := goth_oidc.NewCustomisedURL(
			"client",
			"secret",
			"https://gitea.example/user/oauth2/test-provider/callback",
			"https://login.oidc.example.test/auth",
			"https://login.oidc.example.test/token",
			"https://issuer.oidc.example.test",
			"https://userinfo.oidc.example.test",
			"",
			"openid",
		)
		assert.NoError(t, err)
		provider.SetName("test-provider")
		goth.UseProviders(provider)
		t.Cleanup(func() {
			RemoveProviderFromGothic("test-provider")
		})

		urls := GetSourceEndpointURLs("test-provider", &Source{
			Provider:                      "openidConnect",
			OpenIDConnectAutoDiscoveryURL: "https://discovery.oidc.example.test/.well-known/openid-configuration",
		})

		assert.ElementsMatch(t, []string{
			"https://discovery.oidc.example.test/.well-known/openid-configuration",
			"https://login.oidc.example.test/auth",
			"https://login.oidc.example.test/token",
			"https://issuer.oidc.example.test",
			"https://userinfo.oidc.example.test",
		}, urls)
	})
}
