// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2

import (
	"context"
	"encoding/base64"
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
	t.Run("key", func(t *testing.T) {
		assert.Equal(t, "oauth2_pkce_verifier:my-idp", pkceSessionKey("my-idp"))
	})

	t.Run("challenge_appended", func(t *testing.T) {
		raw := "https://idp.example.com/authorize?client_id=abc&redirect_uri=https%3A%2F%2Ftry.gitea.io%2Fcallback&response_type=code&scope=openid&state=xyz"
		verifier := oauth2.GenerateVerifier()
		out, err := addPKCEChallengeToURL(raw, verifier)
		require.NoError(t, err)
		u, err := url.Parse(out)
		require.NoError(t, err)
		q := u.Query()

		assert.Equal(t, "S256", q.Get("code_challenge_method"))
		// proves challenge == base64url(sha256(verifier)) without hardcoding
		assert.Equal(t, oauth2.S256ChallengeFromVerifier(verifier), q.Get("code_challenge"))
		decoded, err := base64.RawURLEncoding.DecodeString(q.Get("code_challenge"))
		require.NoError(t, err)
		assert.Len(t, decoded, 32) // sha256 length

		// existing params preserved (guards against state loss that breaks gothic state validation)
		assert.Equal(t, "xyz", q.Get("state"))
		assert.Equal(t, "abc", q.Get("client_id"))
		assert.Equal(t, "openid", q.Get("scope"))
		assert.Equal(t, "https://try.gitea.io/callback", q.Get("redirect_uri"))
	})

	t.Run("invalid_url", func(t *testing.T) {
		_, err := addPKCEChallengeToURL("://bad", "v")
		assert.Error(t, err)
	})
}

// TestInjectPKCEVerifier pins the seam between our patch and goth: goth's gothic.CompleteUserAuth reads the
// token-exchange params from request.URL.Query(), so injectPKCEVerifier must land the stored code_verifier
// there (and only there). If a goth bump moves where the verifier is read from, this stays green but real
// logins break, so treat a failure here as "re-verify the goth callback contract".
func TestInjectPKCEVerifier(t *testing.T) {
	const sourceName = "my-idp"

	newReq := func(t *testing.T, store session_module.Store) *http.Request {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "https://try.gitea.io/user/oauth2/my-idp/callback?code=the-code&state=the-state", nil)
		return req.WithContext(context.WithValue(req.Context(), session_module.MockStoreContextKey, store))
	}

	newSource := func(provider, name string) *Source {
		s := &Source{Provider: provider}
		s.SetAuthSource(&auth_model.Source{Name: name})
		return s
	}

	t.Run("oidc verifier reaches goth's params and is single-use", func(t *testing.T) {
		verifier := oauth2.GenerateVerifier()
		store := session_module.NewMockMemStore("test-sid")
		require.NoError(t, store.Set(pkceSessionKey(sourceName), verifier))

		req := newReq(t, store)
		newSource(pkceProvider, sourceName).injectPKCEVerifier(req)

		q := req.URL.Query() // exactly what gothic.CompleteUserAuth reads
		assert.Equal(t, verifier, q.Get("code_verifier"))
		assert.Equal(t, "the-code", q.Get("code")) // callback params preserved
		assert.Equal(t, "the-state", q.Get("state"))
		assert.Nil(t, store.Get(pkceSessionKey(sourceName))) // verifier consumed
	})

	t.Run("no stored verifier is a no-op", func(t *testing.T) {
		req := newReq(t, session_module.NewMockMemStore("test-sid"))
		newSource(pkceProvider, sourceName).injectPKCEVerifier(req)
		assert.Empty(t, req.URL.Query().Get("code_verifier"))
	})

	t.Run("non-oidc provider is untouched", func(t *testing.T) {
		// no session is consulted for non-OIDC providers, so none is provided
		req := httptest.NewRequest(http.MethodGet, "https://try.gitea.io/cb?code=the-code", nil)
		newSource("github", "gh").injectPKCEVerifier(req)
		assert.Empty(t, req.URL.Query().Get("code_verifier"))
	})
}
