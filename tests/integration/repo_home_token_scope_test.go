// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/tests"
)

// TestRepoHomeContentTokenScopes ensures the web repository home page enforces the
// repository read scope (and public-only confinement) of an API token used via basic
// auth, so a wrongly-scoped token cannot read private repository content.
func TestRepoHomeContentTokenScopes(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// user2/repo2 is a private repository owned by user2
	const url = "/user2/repo2"

	// a token without repository scope must be denied
	miscToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadMisc)
	reqDenied := NewRequest(t, "GET", url)
	reqDenied.SetBasicAuth("user2", miscToken)
	MakeRequest(t, reqDenied, http.StatusForbidden)

	// a public-only token must be denied on a private repo
	publicOnlyToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadRepository, auth_model.AccessTokenScopePublicOnly)
	reqPublicOnly := NewRequest(t, "GET", url)
	reqPublicOnly.SetBasicAuth("user2", publicOnlyToken)
	MakeRequest(t, reqPublicOnly, http.StatusForbidden)

	// a token with repository read scope is allowed
	ownerReadToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadRepository)
	reqAllowed := NewRequest(t, "GET", url)
	reqAllowed.SetBasicAuth("user2", ownerReadToken)
	MakeRequest(t, reqAllowed, http.StatusOK)
}
