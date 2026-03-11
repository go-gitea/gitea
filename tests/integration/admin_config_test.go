// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/models/system"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/setting/config"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminConfig(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user1")
	req := NewRequest(t, "GET", "/-/admin/config")
	resp := session.MakeRequest(t, req, http.StatusOK)
	assert.True(t, test.IsNormalPageCompleted(resp.Body.String()))

	t.Run("OpenEditorWithApps", func(t *testing.T) {
		cfg := setting.Config().Repository.OpenWithEditorApps
		editorApps := cfg.Value(t.Context())
		assert.Len(t, editorApps, 3)
		assert.False(t, cfg.HasValue(t.Context()))

		require.NoError(t, system.SetSettings(t.Context(), map[string]string{cfg.DynKey(): "[]"}))
		config.GetDynGetter().InvalidateCache()

		editorApps = cfg.Value(t.Context())
		assert.Len(t, editorApps, 3)
		assert.False(t, cfg.HasValue(t.Context()))

		require.NoError(t, system.SetSettings(t.Context(), map[string]string{cfg.DynKey(): "[{}]"}))
		config.GetDynGetter().InvalidateCache()

		editorApps = cfg.Value(t.Context())
		assert.Len(t, editorApps, 1)
		assert.True(t, cfg.HasValue(t.Context()))
	})

	t.Run("InstanceWebBanner", func(t *testing.T) {
		banner, rev1, has := setting.Config().Instance.WebBanner.ValueRevision(t.Context())
		assert.False(t, has)
		assert.Equal(t, setting.WebBannerType{}, banner)

		req = NewRequestWithValues(t, "POST", "/-/admin/config", map[string]string{
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
