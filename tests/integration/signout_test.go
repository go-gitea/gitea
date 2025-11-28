// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/tests"
)

func TestSignOut_Post(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")

	req := NewRequest(t, "POST", "/user/logout")
	session.MakeRequest(t, req, http.StatusOK)

	// try to view a private repo, should fail
	req = NewRequest(t, "GET", "/user2/repo2")
	session.MakeRequest(t, req, http.StatusNotFound)
}

func TestSignOut_Get(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")

	req := NewRequest(t, "GET", "/user/logout")
	resp := session.MakeRequest(t, req, http.StatusSeeOther)

	location := resp.Header().Get("Location")
	if location != "/" {
		t.Fatalf("expected redirect Location to '/', got %q", location)
	}

	// try to view a private repo, should fail
	req = NewRequest(t, "GET", "/user2/repo2")
	session.MakeRequest(t, req, http.StatusNotFound)
}

func TestSignOut_ReverseProxyLogoutRedirect(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	defer test.MockVariableValue(&setting.ReverseProxyLogoutRedirect, "/mellon/logout?ReturnTo=/user/logout")()

	session := loginUser(t, "user2")

	req := NewRequest(t, "GET", "/")
	resp := session.MakeRequest(t, req, http.StatusOK)

	body := resp.Body.String()

	// check that the external URL is present in the logout button
	if !strings.Contains(body, `href="/mellon/logout?ReturnTo=/user/logout"`) {
		t.Fatalf("logout button does not point to REVERSE_PROXY_LOGOUT_REDIRECT when configured")
	}
}
