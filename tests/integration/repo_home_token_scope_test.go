// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/tests"
)

func TestRepoWebTokenScopes(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	miscToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadMisc)
	publicOnlyToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadRepository, auth_model.AccessTokenScopePublicOnly)
	readToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadRepository)

	for _, test := range []struct {
		name string
		url  string
	}{
		{"repository home", "/user2/repo2"},
		{"workflow badge", "/org3/repo3/actions/workflows/test.yml/badge.svg"},
	} {
		t.Run(test.name, func(t *testing.T) {
			assertBasicAuthStatus(t, test.url, miscToken, http.StatusForbidden)
			assertBasicAuthStatus(t, test.url, publicOnlyToken, http.StatusForbidden)
			assertBasicAuthStatus(t, test.url, readToken, http.StatusOK)
		})
	}

	assertBasicAuthStatus(t, "/user2/repo1/actions/workflows/test.yml/badge.svg", publicOnlyToken, http.StatusOK)
}

func assertBasicAuthStatus(t *testing.T, url, token string, status int) {
	t.Helper()
	req := NewRequest(t, http.MethodGet, url)
	req.SetBasicAuth("user2", token)
	MakeRequest(t, req, status)
}
