// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2_provider

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"math/big"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestECDSASigningKeyToJWK(t *testing.T) {
	for _, tc := range []struct {
		curve         elliptic.Curve
		signingMethod jwt.SigningMethod
		expectedAlg   string
		expectedCrv   string
		coordLen      int
	}{
		{elliptic.P256(), jwt.SigningMethodES256, "ES256", "P-256", 32},
		{elliptic.P384(), jwt.SigningMethodES384, "ES384", "P-384", 48},
		{elliptic.P521(), jwt.SigningMethodES512, "ES512", "P-521", 66},
	} {
		t.Run(tc.expectedCrv, func(t *testing.T) {
			privKey, err := ecdsa.GenerateKey(tc.curve, rand.Reader)
			require.NoError(t, err)

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
			pubKey := privKey.Public().(*ecdsa.PublicKey)
			assert.Equal(t, 0, new(big.Int).SetBytes(xBytes).Cmp(pubKey.X))
			assert.Equal(t, 0, new(big.Int).SetBytes(yBytes).Cmp(pubKey.Y))
		})
	}
}
