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

	// KeySignInIP stores the client IP at sign-in time.
	KeySignInIP = "signInIP"

	// KeySignInTime stores the Unix timestamp at sign-in time.
	KeySignInTime = "signInTime"

	// KeySignInUserAgent stores the raw User-Agent header at sign-in time.
	KeySignInUserAgent = "signInUserAgent"

	// KeyRememberTokenID stores the remember-me auth token ID so it can be
	// revoked together with the session.
	KeyRememberTokenID = "rememberTokenID"
)

// Sign-in method constants for KeySignInMethod.
const (
	SignInMethodPassword = "password"
	SignInMethodRemember = "remember"
)
