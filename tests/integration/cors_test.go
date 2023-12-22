// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/routers"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestCORS(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	t.Run("CORS enabled", func(t *testing.T) {
		defer test.MockVariableValue(&setting.CORSConfig.Enabled, true)()
		defer test.MockVariableValue(&testWebRoutes, routers.NormalRoutes())()

		t.Run("API with CORS", func(t *testing.T) {
			// GET api with no CORS header
			req := NewRequest(t, "GET", "/api/v1/version")
			resp := MakeRequest(t, req, http.StatusOK)
			assert.Empty(t, resp.Header().Get("Access-Control-Allow-Origin"))
			assert.Contains(t, resp.Header().Values("Vary"), "Origin")

			// OPTIONS api for CORS
			req = NewRequest(t, "OPTIONS", "/api/v1/version").
				SetHeader("Origin", "https://example.com").
				SetHeader("Access-Control-Request-Method", "GET")
			resp = MakeRequest(t, req, http.StatusOK)
			assert.NotEmpty(t, resp.Header().Get("Access-Control-Allow-Origin"))
			assert.Contains(t, resp.Header().Values("Vary"), "Origin")
		})

		t.Run("Web with CORS", func(t *testing.T) {
			// GET userinfo with no CORS header
			req := NewRequest(t, "GET", "/login/oauth/userinfo")
			resp := MakeRequest(t, req, http.StatusUnauthorized)
			assert.Empty(t, resp.Header().Get("Access-Control-Allow-Origin"))
			assert.Contains(t, resp.Header().Values("Vary"), "Origin")

			// OPTIONS userinfo for CORS
			req = NewRequest(t, "OPTIONS", "/login/oauth/userinfo").
				SetHeader("Origin", "https://example.com").
				SetHeader("Access-Control-Request-Method", "GET")
			resp = MakeRequest(t, req, http.StatusOK)
			assert.NotEmpty(t, resp.Header().Get("Access-Control-Allow-Origin"))
			assert.Contains(t, resp.Header().Values("Vary"), "Origin")

			// OPTIONS userinfo for non-CORS
			req = NewRequest(t, "OPTIONS", "/login/oauth/userinfo")
			resp = MakeRequest(t, req, http.StatusMethodNotAllowed)
			assert.NotContains(t, resp.Header().Values("Vary"), "Origin")
		})
	})

	t.Run("CORS disabled", func(t *testing.T) {
		defer test.MockVariableValue(&setting.CORSConfig.Enabled, false)()
		defer test.MockVariableValue(&testWebRoutes, routers.NormalRoutes())()

		t.Run("API without CORS", func(t *testing.T) {
			req := NewRequest(t, "GET", "/api/v1/version")
			resp := MakeRequest(t, req, http.StatusOK)
			assert.Empty(t, resp.Header().Get("Access-Control-Allow-Origin"))
			assert.Empty(t, resp.Header().Values("Vary"))

			req = NewRequest(t, "OPTIONS", "/api/v1/version").
				SetHeader("Origin", "https://example.com").
				SetHeader("Access-Control-Request-Method", "GET")
			resp = MakeRequest(t, req, http.StatusMethodNotAllowed)
			assert.Empty(t, resp.Header().Get("Access-Control-Allow-Origin"))
			assert.Empty(t, resp.Header().Values("Vary"))
		})

		t.Run("Web without CORS", func(t *testing.T) {
			req := NewRequest(t, "GET", "/login/oauth/userinfo")
			resp := MakeRequest(t, req, http.StatusUnauthorized)
			assert.Empty(t, resp.Header().Get("Access-Control-Allow-Origin"))
			assert.NotContains(t, resp.Header().Values("Vary"), "Origin")

			req = NewRequest(t, "OPTIONS", "/login/oauth/userinfo").
				SetHeader("Origin", "https://example.com").
				SetHeader("Access-Control-Request-Method", "GET")
			resp = MakeRequest(t, req, http.StatusMethodNotAllowed)
			assert.Empty(t, resp.Header().Get("Access-Control-Allow-Origin"))
			assert.NotContains(t, resp.Header().Values("Vary"), "Origin")
		})
	})
}
