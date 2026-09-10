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

	// KeySignedInAccounts holds the JSON-encoded list of accounts authenticated in this session.
	KeySignedInAccounts = "signedInAccounts"

	// KeyAddingAccountFor holds the uid which started an "add another account" flow.
	KeyAddingAccountFor = "addingAccountFor"
)
