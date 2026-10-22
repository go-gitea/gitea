// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"net/http"
	"testing"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/routers"
	"gitea.dev/tests"
)

func TestPackageReverseProxyAuthUsesAPISetting(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&setting.Service.EnableReverseProxyAuth, true)()

	put := func(t *testing.T, status int) {
		req := NewRequestWithBody(t, "PUT", "/api/packages/user2/generic/artifact/1.0.0/file.bin", bytes.NewReader([]byte("content"))).
			SetHeader(setting.ReverseProxyAuthUser, "user2")
		MakeRequest(t, req, status)
	}

	t.Run("APIDisabled", func(t *testing.T) {
		defer test.MockVariableValue(&setting.Service.EnableReverseProxyAuthAPI, false)()
		defer test.MockVariableValue(&testWebRoutes, routers.NormalRoutes())()
		put(t, http.StatusUnauthorized)
	})

	t.Run("APIEnabled", func(t *testing.T) {
		defer test.MockVariableValue(&setting.Service.EnableReverseProxyAuthAPI, true)()
		defer test.MockVariableValue(&testWebRoutes, routers.NormalRoutes())()
		put(t, http.StatusCreated)
	})
}
