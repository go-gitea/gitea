// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/tests"
)

func TestSignOut(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user2")

	req := NewRequest(t, "POST", "/user/logout")
	resp := session.MakeRequest(t, req, http.StatusSeeOther)

	expected := "/"
	loc := resp.Header().Get("Location")
	if loc != expected {
		t.Fatalf("expected redirect to %q, got %q", expected, loc)
	}

	// try to view a private repo, should fail
	req = NewRequest(t, "GET", "/user2/repo2")
	session.MakeRequest(t, req, http.StatusNotFound)
}

func TestSignOut_ReverseProxyLogoutRedirect(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	defer test.MockVariableValue(&setting.ReverseProxyLogoutRedirect, "/mellon/logout?ReturnTo=/")()

	session := loginUser(t, "user2")

	req := NewRequest(t, "POST", "/user/logout")
	resp := session.MakeRequest(t, req, http.StatusSeeOther)

	expected := "/mellon/logout?ReturnTo=/"
	loc := resp.Header().Get("Location")
	if loc != expected {
		t.Fatalf("expected redirect to %q, got %q", expected, loc)
	}

	// try to view a private repo, should fail
	req = NewRequest(t, "GET", "/user2/repo2")
	session.MakeRequest(t, req, http.StatusNotFound)
}
