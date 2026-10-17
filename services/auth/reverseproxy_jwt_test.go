// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gitea.dev/models/unittest"
	"gitea.dev/modules/reqctx"
	"gitea.dev/modules/session"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/go-jose/go-jose/v4"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetStringClaim(t *testing.T) {
	claims := jwt.MapClaims{
		"sub":  " user2 ",
		"num":  1,
		"user": map[string]any{"login": "nested"},
	}
	assert.Equal(t, "user2", getStringClaim(claims, "sub"))
	assert.Equal(t, "nested", getStringClaim(claims, "user.login"))
	assert.Empty(t, getStringClaim(claims, "num"))
	assert.Empty(t, getStringClaim(claims, "sub.login"))
	assert.Empty(t, getStringClaim(claims, "missing"))
	assert.Empty(t, getStringClaim(claims, ""))
}

func TestReverseProxyJWTParseIdentity(t *testing.T) {
	defer test.MockVariableValue(&setting.ReverseProxyJWT, setting.ReverseProxyJWT)()
	defer test.MockVariableValue(&setting.Service.EnableReverseProxyEmail, true)()
	setting.ReverseProxyJWT.Algorithms = []string{"RS256", "ES256", "EdDSA"}
	setting.ReverseProxyJWT.Issuer = "https://idp.example.com"
	setting.ReverseProxyJWT.Audience = "gitea"
	setting.ReverseProxyJWT.Leeway = 0
	setting.ReverseProxyJWT.UsernameClaim = "preferred_username"
	setting.ReverseProxyJWT.EmailClaim = "email"

	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	edPub, edKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	otherKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	r := &ReverseProxyJWT{keys: &jwtKeyProvider{keys: []jose.JSONWebKey{
		{Key: &rsaKey.PublicKey, KeyID: "rsa"},
		{Key: &ecKey.PublicKey, KeyID: "ec"},
		{Key: edPub, KeyID: "ed"},
	}}}

	validClaims := func() jwt.MapClaims {
		return jwt.MapClaims{
			"iss":                "https://idp.example.com",
			"aud":                "gitea",
			"exp":                time.Now().Add(time.Minute).Unix(),
			"preferred_username": "user2",
			"email":              "user2@example.com",
		}
	}
	sign := func(method jwt.SigningMethod, kid string, key crypto.PrivateKey, claims jwt.MapClaims) string {
		token := jwt.NewWithClaims(method, claims)
		if kid != "" {
			token.Header["kid"] = kid
		}
		s, err := token.SignedString(key)
		require.NoError(t, err)
		return s
	}

	for _, tc := range []struct {
		name   string
		method jwt.SigningMethod
		kid    string
		key    crypto.PrivateKey
	}{
		{"RS256", jwt.SigningMethodRS256, "rsa", rsaKey},
		{"ES256", jwt.SigningMethodES256, "ec", ecKey},
		{"EdDSA", jwt.SigningMethodEdDSA, "ed", edKey},
		{"no kid", jwt.SigningMethodES256, "", ecKey},
	} {
		t.Run("valid "+tc.name, func(t *testing.T) {
			id, err := r.parseIdentity(t.Context(), sign(tc.method, tc.kid, tc.key, validClaims()))
			require.NoError(t, err)
			assert.Equal(t, reverseProxyIdentity{Username: "user2", Email: "user2@example.com"}, id)
		})
	}

	rsaPubDER, err := x509.MarshalPKIXPublicKey(&rsaKey.PublicKey)
	require.NoError(t, err)

	invalid := map[string]func() string{
		"expired": func() string {
			c := validClaims()
			c["exp"] = time.Now().Add(-time.Minute).Unix()
			return sign(jwt.SigningMethodES256, "ec", ecKey, c)
		},
		"missing exp": func() string {
			c := validClaims()
			delete(c, "exp")
			return sign(jwt.SigningMethodES256, "ec", ecKey, c)
		},
		"not yet valid": func() string {
			c := validClaims()
			c["nbf"] = time.Now().Add(time.Minute).Unix()
			return sign(jwt.SigningMethodES256, "ec", ecKey, c)
		},
		"wrong issuer": func() string {
			c := validClaims()
			c["iss"] = "https://evil.example.com"
			return sign(jwt.SigningMethodES256, "ec", ecKey, c)
		},
		"wrong audience": func() string {
			c := validClaims()
			c["aud"] = "other"
			return sign(jwt.SigningMethodES256, "ec", ecKey, c)
		},
		"missing username": func() string {
			c := validClaims()
			delete(c, "preferred_username")
			return sign(jwt.SigningMethodES256, "ec", ecKey, c)
		},
		"untrusted key": func() string {
			return sign(jwt.SigningMethodES256, "ec", otherKey, validClaims())
		},
		"unknown kid": func() string {
			return sign(jwt.SigningMethodES256, "nope", ecKey, validClaims())
		},
		"alg not allowed": func() string {
			return sign(jwt.SigningMethodRS512, "rsa", rsaKey, validClaims())
		},
		"alg none": func() string {
			return sign(jwt.SigningMethodNone, "", jwt.UnsafeAllowNoneSignatureType, validClaims())
		},
		"HS256 signed with public key": func() string {
			return sign(jwt.SigningMethodHS256, "rsa", rsaPubDER, validClaims())
		},
		"malformed": func() string {
			return "not.a.jwt"
		},
	}
	for name, token := range invalid {
		t.Run(name, func(t *testing.T) {
			_, err := r.parseIdentity(t.Context(), token())
			assert.Error(t, err)
		})
	}
}

func TestReverseProxyJWTVerify(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	defer test.MockVariableValue(&setting.ReverseProxyJWT, setting.ReverseProxyJWT)()
	setting.ReverseProxyJWT.Header = "X-JWT-Assertion"
	setting.ReverseProxyJWT.Algorithms = []string{"ES256"}
	setting.ReverseProxyJWT.UsernameClaim = "sub"

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	r := &ReverseProxyJWT{keys: &jwtKeyProvider{keys: []jose.JSONWebKey{{Key: &key.PublicKey}}}}
	sign := func(sub string) string {
		s, err := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
			"sub": sub,
			"exp": time.Now().Add(time.Minute).Unix(),
		}).SignedString(key)
		require.NoError(t, err)
		return s
	}
	verify := func(header string, sess SessionStore) (reqctx.ContextData, int64, error) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if header != "" {
			req.Header.Set("X-JWT-Assertion", header)
		}
		store := reqctx.ContextData{}
		user, err := r.Verify(req, httptest.NewRecorder(), store, sess)
		if user != nil {
			return store, user.ID, err
		}
		return store, 0, err
	}

	t.Run("no header", func(t *testing.T) {
		_, uid, err := verify("", nil)
		assert.NoError(t, err)
		assert.Zero(t, uid)
	})

	t.Run("valid token with bearer prefix", func(t *testing.T) {
		store, uid, err := verify("Bearer "+sign("user2"), nil)
		require.NoError(t, err)
		assert.EqualValues(t, 2, uid)
		assert.Equal(t, true, store["IsReverseProxy"])
	})

	t.Run("invalid token clears session", func(t *testing.T) {
		sess := session.NewMockMemStore("sid")
		require.NoError(t, sess.Set(session.KeyUID, int64(2)))
		_, uid, err := verify(sign("user2")+"x", sess)
		assert.Error(t, err)
		assert.Zero(t, uid)
		assert.Nil(t, sess.Get(session.KeyUID))
	})

	t.Run("unknown user clears session", func(t *testing.T) {
		defer test.MockVariableValue(&setting.Service.EnableReverseProxyAutoRegister, false)()
		sess := session.NewMockMemStore("sid")
		require.NoError(t, sess.Set(session.KeyUID, int64(2)))
		_, _, err := verify(sign("no-such-user"), sess)
		assert.Error(t, err)
		assert.Nil(t, sess.Get(session.KeyUID))
	})

	t.Run("auto registration", func(t *testing.T) {
		defer test.MockVariableValue(&setting.Service.EnableReverseProxyAutoRegister, true)()
		_, uid, err := verify(sign("jwt-new-user"), nil)
		require.NoError(t, err)
		assert.NotZero(t, uid)
	})
}
