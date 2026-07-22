// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"gitea.dev/models/system"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/setting/config"
	"gitea.dev/modules/test"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminConfig(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user1")

	t.Run("ConfigPage", func(t *testing.T) {
		req := NewRequest(t, "GET", "/-/admin/config")
		resp := session.MakeRequest(t, req, http.StatusOK)
		assert.True(t, test.IsNormalPageCompleted(resp.Body.String()))
	})

	t.Run("OpenEditorWithApps", func(t *testing.T) {
		cfg := setting.Config().Repository.OpenWithEditorApps

		t.Run("Default", func(t *testing.T) {
			editorApps := cfg.Value(t.Context())
			assert.Len(t, editorApps, 3)
			assert.False(t, cfg.HasValue(t.Context()))
		})

		t.Run("EmptyAsDefault", func(t *testing.T) {
			require.NoError(t, system.SetSettings(t.Context(), map[string]string{cfg.DynKey(): "[]"}))
			config.GetDynGetter().InvalidateCache()

			editorApps := cfg.Value(t.Context())
			assert.Len(t, editorApps, 3)
			assert.False(t, cfg.HasValue(t.Context()))
		})

		t.Run("SingleItem", func(t *testing.T) {
			require.NoError(t, system.SetSettings(t.Context(), map[string]string{cfg.DynKey(): "[{}]"}))
			config.GetDynGetter().InvalidateCache()

			editorApps := cfg.Value(t.Context())
			assert.Len(t, editorApps, 1)
			assert.True(t, cfg.HasValue(t.Context()))
		})

		t.Run("ManualSet", func(t *testing.T) {
			req := NewRequestWithValues(t, "POST", "/-/admin/config", map[string]string{
				"key":   "repository.open-with.editor-apps",
				"value": `[{"DisplayName":"app-name","OpenURL":"my-app:?u={url}"}]`,
			})
			session.MakeRequest(t, req, http.StatusOK)
			editorApps := cfg.Value(t.Context())
			assert.Len(t, editorApps, 1)
			assert.Equal(t, "app-name", editorApps[0].DisplayName)
			assert.Equal(t, "my-app:?u={url}", editorApps[0].OpenURL)
			assert.True(t, cfg.HasValue(t.Context()))
		})
	})

	t.Run("InstanceWebBanner", func(t *testing.T) {
		banner, rev1, has := setting.Config().Instance.WebBanner.ValueRevision(t.Context())
		assert.False(t, has)
		assert.Equal(t, setting.WebBannerType{}, banner)

		req := NewRequestWithValues(t, "POST", "/-/admin/config", map[string]string{
			"key":   "instance.web_banner",
			"value": `{"DisplayEnabled":true,"ContentMessage":"test-msg","StartTimeUnix":123,"EndTimeUnix":456}`,
		})
		session.MakeRequest(t, req, http.StatusOK)
		banner, rev2, has := setting.Config().Instance.WebBanner.ValueRevision(t.Context())
		assert.NotEqual(t, rev1, rev2)
		assert.True(t, has)
		assert.Equal(t, setting.WebBannerType{
			DisplayEnabled: true,
			ContentMessage: "test-msg",
			StartTimeUnix:  123,
			EndTimeUnix:    456,
		}, banner)
	})
}
