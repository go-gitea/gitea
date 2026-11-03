// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/markbates/goth/providers/openidConnect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetProviderForRequestAutoCallbackURL(t *testing.T) {
	oldAppURL := setting.AppURL
	setting.AppURL = "http://localhost:3000/"
	defer func() {
		setting.AppURL = oldAppURL
		ClearProviders()
	}()

	t.Run("StandardOAuth2Provider", func(t *testing.T) {
		ClearProviders()
		src := &Source{
			Provider:        "github",
			ClientID:        "test-client-id",
			ClientSecret:    "test-client-secret",
			AutoCallbackURL: false,
		}
		require.NoError(t, RegisterProviderWithGothic("my-github", src))

		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost:3000/user/oauth2/my-github", nil)
		require.NoError(t, err)
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Header.Set("X-Forwarded-Host", "git.example.com")

		// 1. AutoCallbackURL disabled -> uses static setting.AppURL
		pDisabled, err := getProviderForRequest("my-github", req, src)
		require.NoError(t, err)
		sessDisabled, err := pDisabled.BeginAuth("state1")
		require.NoError(t, err)
		authURLDisabled, err := sessDisabled.GetAuthURL()
		require.NoError(t, err)
		assert.Contains(t, authURLDisabled, "redirect_uri=http%3A%2F%2Flocalhost%3A3000%2Fuser%2Foauth2%2Fmy-github%2Fcallback")

		// 2. AutoCallbackURL enabled -> derives callback URL from current request headers
		src.AutoCallbackURL = true
		pEnabled, err := getProviderForRequest("my-github", req, src)
		require.NoError(t, err)
		assert.Equal(t, "my-github", pEnabled.Name())
		sessEnabled, err := pEnabled.BeginAuth("state2")
		require.NoError(t, err)
		authURLEnabled, err := sessEnabled.GetAuthURL()
		require.NoError(t, err)
		assert.Contains(t, authURLEnabled, "redirect_uri=https%3A%2F%2Fgit.example.com%2Fuser%2Foauth2%2Fmy-github%2Fcallback")
	})

	t.Run("OpenIDConnectReusesDiscoveredEndpoints", func(t *testing.T) {
		ClearProviders()
		discoveryHits := 0
		var tsURL string
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			discoveryHits++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"issuer": "` + tsURL + `",
				"authorization_endpoint": "` + tsURL + `/authorize",
				"token_endpoint": "` + tsURL + `/token",
				"userinfo_endpoint": "` + tsURL + `/userinfo"
			}`))
		}))
		defer ts.Close()
		tsURL = ts.URL

		src := &Source{
			Provider:                      "openidConnect",
			ClientID:                      "oidc-client-id",
			ClientSecret:                  "oidc-client-secret",
			OpenIDConnectAutoDiscoveryURL: ts.URL + "/.well-known/openid-configuration",
			AutoCallbackURL:               true,
		}
		require.NoError(t, RegisterProviderWithGothic("my-oidc", src))
		assert.Equal(t, 1, discoveryHits)

		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost:3000/user/oauth2/my-oidc", nil)
		require.NoError(t, err)
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Header.Set("X-Forwarded-Host", "sso.example.com")

		p, err := getProviderForRequest("my-oidc", req, src)
		require.NoError(t, err)
		// Must NOT make a second HTTP discovery call on per-request clone
		assert.Equal(t, 1, discoveryHits)

		oidcP, ok := p.(*openidConnect.Provider)
		require.True(t, ok)
		assert.Equal(t, "https://sso.example.com/user/oauth2/my-oidc/callback", oidcP.CallbackURL)

		sess, err := p.BeginAuth("state-oidc")
		require.NoError(t, err)
		authURL, err := sess.GetAuthURL()
		require.NoError(t, err)
		assert.Contains(t, authURL, "redirect_uri=https%3A%2F%2Fsso.example.com%2Fuser%2Foauth2%2Fmy-oidc%2Fcallback")
	})
}
