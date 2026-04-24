// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestSignOut(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	t.Run("NormalLogout", func(t *testing.T) {
		session := loginUser(t, "user2")

		req := NewRequest(t, "GET", "/user/logout")
		resp := session.MakeRequest(t, req, http.StatusSeeOther)
		assert.Equal(t, "/", resp.Header().Get("Location"))

		// logged out, try to view a private repo, should fail
		req = NewRequest(t, "GET", "/user2/repo2")
		session.MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("ReverseProxyLogoutRedirect", func(t *testing.T) {
		defer test.MockVariableValue(&setting.Service.EnableReverseProxyAuth, true)()
		defer test.MockVariableValue(&setting.ReverseProxyLogoutRedirect, "/my-sso/logout?return_to=/my-sso/home")()

		session := loginUser(t, "user2")
		req := NewRequest(t, "GET", "/user/logout")
		resp := session.MakeRequest(t, req, http.StatusSeeOther)
		assert.Equal(t, "/my-sso/logout?return_to=/my-sso/home", resp.Header().Get("Location"))

		// logged out, try to view a private repo, should fail
		req = NewRequest(t, "GET", "/user2/repo2")
		session.MakeRequest(t, req, http.StatusNotFound)
	})
}
