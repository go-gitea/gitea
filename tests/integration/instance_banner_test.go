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

func TestInstanceBanner(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	setInstanceBanner := func(t *testing.T, banner setting.InstanceBannerType) {
		t.Helper()
		marshaled, err := json.Marshal(banner)
		require.NoError(t, err)
		require.NoError(t, system_model.SetSettings(t.Context(), map[string]string{
			setting.Config().WebUI.InstanceBanner.DynKey(): string(marshaled),
		}))
		config.GetDynGetter().InvalidateCache()
	}

	t.Run("Visibility", func(t *testing.T) {
		setInstanceBanner(t, setting.InstanceBannerType{
			DisplayEnabled: true,
			ContentMessage: "Planned **upgrade** in progress.",
		})

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
		setInstanceBanner(t, setting.InstanceBannerType{
			DisplayEnabled: true,
			ContentMessage: "Future banner",
			StartTimeUnix:  now + 3600,
			EndTimeUnix:    now + 7200,
		})

		resp := MakeRequest(t, NewRequest(t, "GET", "/"), http.StatusOK)
		assert.NotContains(t, resp.Body.String(), "Future banner")

		setInstanceBanner(t, setting.InstanceBannerType{
			DisplayEnabled: true,
			ContentMessage: "Expired banner",
			StartTimeUnix:  now - 7200,
			EndTimeUnix:    now - 3600,
		})

		resp = MakeRequest(t, NewRequest(t, "GET", "/"), http.StatusOK)
		assert.NotContains(t, resp.Body.String(), "Expired banner")
	})
}
