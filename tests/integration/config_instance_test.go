// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"
	"time"

	system_model "code.gitea.io/gitea/models/system"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/setting/config"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mockSystemConfig[T any](t *testing.T, opt *config.Option[T], v T) func() {
	jsonBuf, _ := json.Marshal(v)
	old := opt.Value(t.Context())
	require.NoError(t, system_model.SetSettings(t.Context(), map[string]string{opt.DynKey(): string(jsonBuf)}))
	config.GetDynGetter().InvalidateCache()
	return func() {
		jsonBuf, _ := json.Marshal(old)
		require.NoError(t, system_model.SetSettings(t.Context(), map[string]string{opt.DynKey(): string(jsonBuf)}))
		config.GetDynGetter().InvalidateCache()
	}
}

func TestInstance(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	t.Run("WebBanner", func(t *testing.T) {
		t.Run("Visibility", func(t *testing.T) {
			defer mockSystemConfig(t, setting.Config().Instance.WebBanner, setting.WebBannerType{
				DisplayEnabled: true,
				ContentMessage: "Planned **upgrade** in progress.",
			})()

			t.Run("AnonymousUserSeesBanner", func(t *testing.T) {
				resp := MakeRequest(t, NewRequest(t, "GET", "/"), http.StatusOK)
				assert.Contains(t, resp.Body.String(), "Planned <strong>upgrade</strong> in progress.")
			})

			t.Run("NormalUserSeesBanner", func(t *testing.T) {
				sess := loginUser(t, "user2")
				resp := sess.MakeRequest(t, NewRequest(t, "GET", "/user/settings"), http.StatusOK)
				assert.Contains(t, resp.Body.String(), "Planned <strong>upgrade</strong> in progress.")
			})

			t.Run("AdminSeesBannerWithoutEditHint", func(t *testing.T) {
				sess := loginUser(t, "user1")
				resp := sess.MakeRequest(t, NewRequest(t, "GET", "/-/admin"), http.StatusOK)
				assert.Contains(t, resp.Body.String(), "Planned <strong>upgrade</strong> in progress.")
				assert.NotContains(t, resp.Body.String(), "Edit this banner")
			})

			t.Run("APIRequestUnchanged", func(t *testing.T) {
				MakeRequest(t, NewRequest(t, "GET", "/api/v1/version"), http.StatusOK)
			})
		})

		t.Run("TimeWindow", func(t *testing.T) {
			now := time.Now().Unix()
			defer mockSystemConfig(t, setting.Config().Instance.WebBanner, setting.WebBannerType{
				DisplayEnabled: true,
				ContentMessage: "Future banner",
				StartTimeUnix:  now + 3600,
				EndTimeUnix:    now + 7200,
			})()

			resp := MakeRequest(t, NewRequest(t, "GET", "/"), http.StatusOK)
			assert.NotContains(t, resp.Body.String(), "Future banner")

			defer mockSystemConfig(t, setting.Config().Instance.WebBanner, setting.WebBannerType{
				DisplayEnabled: true,
				ContentMessage: "Expired banner",
				StartTimeUnix:  now - 7200,
				EndTimeUnix:    now - 3600,
			})()

			resp = MakeRequest(t, NewRequest(t, "GET", "/"), http.StatusOK)
			assert.NotContains(t, resp.Body.String(), "Expired banner")
		})
	})

	t.Run("MaintenanceMode", func(t *testing.T) {
		defer mockSystemConfig(t, setting.Config().Instance.WebBanner, setting.WebBannerType{
			DisplayEnabled: true,
			ContentMessage: "MaintenanceModeBanner",
		})()
		defer mockSystemConfig(t, setting.Config().Instance.MaintenanceMode, setting.MaintenanceModeType{AdminWebAccessOnly: true})()

		t.Run("AnonymousUser", func(t *testing.T) {
			req := NewRequest(t, "GET", "/")
			req.Header.Add("Accept", "text/html")
			resp := MakeRequest(t, req, http.StatusServiceUnavailable)
			assert.Contains(t, resp.Body.String(), "MaintenanceModeBanner")
			assert.Contains(t, resp.Body.String(), `href="/user/login"`) // it must contain the login link

			MakeRequest(t, NewRequest(t, "GET", "/user/login"), http.StatusOK)
			MakeRequest(t, NewRequest(t, "GET", "/-/admin"), http.StatusSeeOther)
			MakeRequest(t, NewRequest(t, "GET", "/api/internal/dummy"), http.StatusForbidden)
		})

		t.Run("AdminLogin", func(t *testing.T) {
			req := NewRequestWithValues(t, "POST", "/user/login", map[string]string{"user_name": "user1", "password": userPassword})
			resp := MakeRequest(t, req, http.StatusSeeOther)
			assert.Equal(t, "/-/admin", resp.Header().Get("Location"))

			sess := loginUser(t, "user1")
			req = NewRequest(t, "GET", "/")
			req.Header.Add("Accept", "text/html")
			resp = sess.MakeRequest(t, req, http.StatusServiceUnavailable)
			assert.Contains(t, resp.Body.String(), "MaintenanceModeBanner")

			resp = sess.MakeRequest(t, NewRequest(t, "GET", "/user/login"), http.StatusSeeOther)
			assert.Equal(t, "/-/admin", resp.Header().Get("Location"))

			sess.MakeRequest(t, NewRequest(t, "GET", "/-/admin"), http.StatusOK)
		})
	})
}
