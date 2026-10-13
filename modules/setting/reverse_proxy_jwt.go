// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"slices"

	"gitea.dev/modules/log"
)

// ReverseProxyJWTAlgorithms are the asymmetric signing algorithms accepted for reverse proxy JWTs.
// Symmetric (HS*) and "none" are deliberately excluded.
var ReverseProxyJWTAlgorithms = []string{"RS256", "RS384", "RS512", "PS256", "PS384", "PS512", "ES256", "ES384", "ES512", "EdDSA"}

func checkReverseProxyJWTSettings() {
	if !Service.EnableReverseProxyJWTAuth && !Service.EnableReverseProxyJWTAuthAPI {
		return
	}
	if Service.EnableReverseProxyAuth || Service.EnableReverseProxyAuthAPI {
		log.Warn("Both header based and JWT based reverse proxy authentication are enabled, header based authentication trusts unverified request headers")
	}

	sources := 0
	for _, s := range []string{ReverseProxyJWT.JWKSetURL, ReverseProxyJWT.JWKSetFile, ReverseProxyJWT.KeyFile} {
		if s != "" {
			sources++
		}
	}
	if sources != 1 {
		log.Fatal("Exactly one of REVERSE_PROXY_AUTHENTICATION_JWT_JWK_SET_URL, REVERSE_PROXY_AUTHENTICATION_JWT_JWK_SET_FILE or REVERSE_PROXY_AUTHENTICATION_JWT_KEY_FILE must be set when JWT reverse proxy authentication is enabled")
	}
	for _, alg := range ReverseProxyJWT.Algorithms {
		if !slices.Contains(ReverseProxyJWTAlgorithms, alg) {
			log.Fatal("Unsupported REVERSE_PROXY_AUTHENTICATION_JWT_ALGORITHMS value %q, supported: %v", alg, ReverseProxyJWTAlgorithms)
		}
	}
	if ReverseProxyJWT.Header == "" || ReverseProxyJWT.UsernameClaim == "" {
		log.Fatal("REVERSE_PROXY_AUTHENTICATION_JWT_HEADER and REVERSE_PROXY_AUTHENTICATION_JWT_USERNAME_CLAIM must not be empty")
	}
}
