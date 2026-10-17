// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/tests"
)

func TestReverseProxyAuthTrustedPeer(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&setting.Service.EnableReverseProxyAuth, true)()

	t.Run("DirectClient", func(t *testing.T) {
		req := NewRequest(t, "GET", "/user/settings").SetHeader("X-WEBAUTH-USER", "user2")
		req.RemoteAddr = "203.0.113.7:1234"
		MakeRequest(t, req, http.StatusSeeOther)
	})

	t.Run("TrustedProxy", func(t *testing.T) {
		req := NewRequest(t, "GET", "/user/settings").SetHeader("X-WEBAUTH-USER", "user2").SetHeader("X-Real-IP", "203.0.113.7")
		req.RemoteAddr = "127.0.0.1:1234"
		MakeRequest(t, req, http.StatusOK)
	})
}
