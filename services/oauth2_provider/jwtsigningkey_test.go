// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2_provider

import (
	"crypto/ecdsa"
	"encoding/base64"
	"math/big"
	"path/filepath"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestECDSASigningKeyToJWK(t *testing.T) {
	for _, tc := range []struct {
		signingMethod jwt.SigningMethod
		expectedAlg   string
		expectedCrv   string
		coordLen      int
	}{
		{jwt.SigningMethodES256, "ES256", "P-256", 32},
		{jwt.SigningMethodES384, "ES384", "P-384", 48},
		{jwt.SigningMethodES512, "ES512", "P-521", 66},
	} {
		t.Run(tc.expectedCrv, func(t *testing.T) {
			key, err := LoadOrCreateAsymmetricKey(filepath.Join(t.TempDir(), "key.pem"), tc.expectedAlg)
			require.NoError(t, err)
			privKey, ok := key.(*ecdsa.PrivateKey)
			require.True(t, ok)

			signingKey, err := newECDSASingingKey(tc.signingMethod, privKey)
			require.NoError(t, err)

			jwk, err := signingKey.ToJWK()
			require.NoError(t, err)

			assert.Equal(t, "EC", jwk["kty"])
			assert.Equal(t, tc.expectedAlg, jwk["alg"])
			assert.Equal(t, tc.expectedCrv, jwk["crv"])
			assert.NotEmpty(t, jwk["kid"])

			// Verify coordinates are the correct fixed length per RFC 7518 / SEC 1
			xBytes, err := base64.RawURLEncoding.DecodeString(jwk["x"])
			require.NoError(t, err)
			assert.Len(t, xBytes, tc.coordLen)

			yBytes, err := base64.RawURLEncoding.DecodeString(jwk["y"])
			require.NoError(t, err)
			assert.Len(t, yBytes, tc.coordLen)

			// Verify the decoded coordinates reconstruct the original public key point
			pubKey, ok := privKey.Public().(*ecdsa.PublicKey)
			require.True(t, ok)
			assert.Equal(t, 0, new(big.Int).SetBytes(xBytes).Cmp(pubKey.X))
			assert.Equal(t, 0, new(big.Int).SetBytes(yBytes).Cmp(pubKey.Y))
		})
	}
}

func TestGrantAdditionalScopes(t *testing.T) {
	tests := []struct {
		grantScopes    string
		expectedScopes string
	}{
		{"", "all"}, // for old tokens without scope, treat it as "all"
		{"openid profile email", "all"},
		{"openid profile email groups", "all"},
		{"openid profile email all", "all"},
		{"openid profile email read:user all", "all"},
		{"openid profile email groups read:user", "read:user"},
		{"read:user read:repository", "read:repository,read:user"},
		{"read:user write:issue public-only", "public-only,write:issue,read:user"},
		{"openid profile email read:user", "read:user"},
		{"no-token-scope", "no-token-scope"},
		{"openid no-token-scope", "no-token-scope"},
		{"no-token-scope read:user", "read:user"},
		{"read:invalid_scope", "no-token-scope"},
		{"read:invalid_scope,write:scope_invalid,just-plain-wrong", "no-token-scope"},
		{"openid invalid:scope", "no-token-scope"},
		{"openid ,,", "no-token-scope"},
		{"read:user invalid:scope", "read:user"},
		{"read:user,invalid:scope public-only", "public-only,read:user"},
	}

	for _, test := range tests {
		t.Run("scope:"+test.grantScopes, func(t *testing.T) {
			result := GrantAdditionalScopes(test.grantScopes)
			assert.Equal(t, test.expectedScopes, string(result))
		})
	}
}
