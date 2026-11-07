// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	auth_model "gitea.dev/models/auth"
	session_module "gitea.dev/modules/session"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func TestPKCEChallenge(t *testing.T) {
	t.Run("challenge_appended", func(t *testing.T) {
		verifier := oauth2.GenerateVerifier()
		authURL, err := addPKCEChallengeToURL("https://idp.example.com/authorize?client_id=abc&redirect_uri=https%3A%2F%2Ftry.gitea.io%2Fcallback&response_type=code&scope=openid&state=xyz", verifier)
		require.NoError(t, err)
		parsedURL, err := url.Parse(authURL)
		require.NoError(t, err)
		query := parsedURL.Query()

		assert.Equal(t, "S256", query.Get("code_challenge_method"))
		assert.Equal(t, oauth2.S256ChallengeFromVerifier(verifier), query.Get("code_challenge"))
		assert.Equal(t, "xyz", query.Get("state"))
		assert.Equal(t, "abc", query.Get("client_id"))
		assert.Equal(t, "openid", query.Get("scope"))
		assert.Equal(t, "https://try.gitea.io/callback", query.Get("redirect_uri"))
	})

	t.Run("invalid_url", func(t *testing.T) {
		_, err := addPKCEChallengeToURL("://bad", "v")
		assert.Error(t, err)
	})
}

func TestInjectPKCEVerifier(t *testing.T) {
	const sourceName = "my-idp"
	const callbackURL = "https://try.gitea.io/user/oauth2/my-idp/callback?code=the-code&state=the-state"
	for _, testCase := range []struct {
		name     string
		provider string
		verifier string
	}{
		{name: "oidc verifier reaches goth without changing the shared URL and is single-use", provider: "openidConnect", verifier: oauth2.GenerateVerifier()},
		{name: "no stored verifier is a no-op", provider: "openidConnect"},
		{name: "non-oidc provider needs no session", provider: "github"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			source := &Source{Provider: testCase.provider}
			source.SetAuthSource(&auth_model.Source{Name: sourceName})
			request := httptest.NewRequest(http.MethodGet, callbackURL, nil)
			store := session_module.NewMockMemStore("test-sid")
			if testCase.provider == "openidConnect" {
				if testCase.verifier != "" {
					require.NoError(t, store.Set(pkceSessionKey(sourceName), testCase.verifier))
				}
				request = request.WithContext(context.WithValue(request.Context(), session_module.MockStoreContextKey, store))
			}

			gothRequest := source.injectPKCEVerifier(request)
			assert.Equal(t, testCase.verifier, gothRequest.URL.Query().Get("code_verifier"))
			assert.Equal(t, "the-code", gothRequest.URL.Query().Get("code"))
			assert.Equal(t, "the-state", gothRequest.URL.Query().Get("state"))
			assert.Equal(t, callbackURL, request.URL.String())
			assert.Nil(t, store.Get(pkceSessionKey(sourceName)))
		})
	}
}
