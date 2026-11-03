// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2

import (
	"net/http"
	"testing"
	"time"

	"gitea.dev/modules/httplib"
	"gitea.dev/modules/reqctx"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/markbates/goth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestGetOAuth2CallbackURL(t *testing.T) {
	defer test.MockVariableValue(&setting.AppURL, "http://cfg-host:3000/sub/")()
	defer test.MockVariableValue(&setting.AppSubURL, "/sub")()
	defer test.MockVariableValue(&setting.PublicURLDetection, setting.PublicURLAuto)()

	reqCtx := reqctx.NewRequestContextForTest(t)
	req := &http.Request{
		Host:   "lan-host:3000",
		Header: http.Header{"X-Forwarded-Proto": {"https"}},
	}
	_ = httplib.RequestWithContext(req, reqCtx)
	httplib.MarkRequestSupportPublicURL(reqCtx)

	cbURL := GetOAuth2CallbackURL(reqCtx, "my-oidc")
	assert.Equal(t, "https://lan-host:3000/sub/user/oauth2/my-oidc/callback", cbURL)

	provider, err := createProvider("my-oidc", cbURL, &Source{
		Provider:     "gitea",
		ClientID:     "client",
		ClientSecret: "secret",
	})
	require.NoError(t, err)
	sess, err := provider.BeginAuth("state")
	require.NoError(t, err)
	authURL, err := sess.GetAuthURL()
	require.NoError(t, err)
	assert.Contains(t, authURL, "redirect_uri=https%3A%2F%2Flan-host%3A3000%2Fsub%2Fuser%2Foauth2%2Fmy-oidc%2Fcallback")
}
