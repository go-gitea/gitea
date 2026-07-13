// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package session

const (
	KeyUID   = "uid"
	KeyUname = "uname"

	KeyUserHasTwoFactorAuth = "userHasTwoFactorAuth"

	// KeySignInMethod records how the current session was authenticated so logout
	// can decide whether RP-initiated OIDC logout is appropriate.
	KeySignInMethod = "signInMethod"

	SignInMethodPassword = "password"
	SignInMethodOAuth2   = "oauth2"
)
