// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	auth_service "gitea.dev/services/auth"
	"gitea.dev/tests"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

func TestSignInReverseProxyJWT(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	keyFile := filepath.Join(t.TempDir(), "jwt.pem")
	require.NoError(t, os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), 0o600))

	defer test.MockVariableValue(&setting.Service.EnableReverseProxyJWTAuth, true)()
	defer test.MockVariableValue(&setting.ReverseProxyJWT, setting.ReverseProxyJWT)()
	setting.ReverseProxyJWT.Header = "X-JWT-Assertion"
	setting.ReverseProxyJWT.KeyFile = keyFile
	setting.ReverseProxyJWT.Algorithms = []string{"ES256"}
	setting.ReverseProxyJWT.UsernameClaim = "sub"
	auth_service.Init() // loads the verification keys from settings

	sign := func(signer *ecdsa.PrivateKey, sub string) string {
		s, err := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
			"sub": sub,
			"exp": time.Now().Add(time.Minute).Unix(),
		}).SignedString(signer)
		require.NoError(t, err)
		return s
	}

	// user2/repo2 is private, so it is only visible when signed in as user2
	req := NewRequest(t, "GET", "/user2/repo2").SetHeader("X-JWT-Assertion", sign(key, "user2"))
	MakeRequest(t, req, http.StatusOK)

	// the plain header method is disabled, so its header must be ignored
	req = NewRequest(t, "GET", "/user2/repo2").SetHeader("X-WEBAUTH-USER", "user2")
	MakeRequest(t, req, http.StatusNotFound)

	otherKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	req = NewRequest(t, "GET", "/user2/repo2").SetHeader("X-JWT-Assertion", sign(otherKey, "user2"))
	MakeRequest(t, req, http.StatusUnauthorized)

	// an invalid token must not fall back to an existing session
	session := loginUser(t, "user2")
	req = NewRequest(t, "GET", "/user2/repo2").SetHeader("X-JWT-Assertion", sign(otherKey, "user2"))
	session.MakeRequest(t, req, http.StatusUnauthorized)
	session.MakeRequest(t, NewRequest(t, "GET", "/user2/repo2"), http.StatusNotFound)
}
