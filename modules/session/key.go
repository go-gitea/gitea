// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package session

const (
	KeyUID = "uid"

	KeyImpersonatorData = "impersonatorData"

	KeyUserHasTwoFactorAuth = "userHasTwoFactorAuth"

	// KeySignInMethod records how the current session was authenticated so logout
	// can decide whether RP-initiated OIDC logout is appropriate.
	KeySignInMethod = "signInMethod"

	SignInMethodOAuth2 = "oauth2"

	// KeyOIDCIDToken holds the raw id_token returned by the OIDC provider at sign-in,
	// so logout can pass it back as id_token_hint per the RP-Initiated Logout spec.
	KeyOIDCIDToken = "oidcIDToken"
)
