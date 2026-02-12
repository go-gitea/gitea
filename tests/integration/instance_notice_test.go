// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"strings"
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

func TestInstanceNoticeVisibility(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	setInstanceNoticeForTest(t, setting.DefaultInstanceNotice())

	setInstanceNoticeForTest(t, setting.InstanceNotice{
		Enabled: true,
		Message: "Planned **upgrade** in progress.",
		Level:   setting.InstanceNoticeLevelWarning,
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

	t.Run("AdminSeesBannerAndEditHint", func(t *testing.T) {
		sess := loginUser(t, "user1")
		resp := sess.MakeRequest(t, NewRequest(t, "GET", "/-/admin"), http.StatusOK)
		assert.Contains(t, resp.Body.String(), "Planned <strong>upgrade</strong> in progress.")
		assert.Contains(t, resp.Body.String(), "Edit this banner")
	})

	t.Run("APIRequestUnchanged", func(t *testing.T) {
		MakeRequest(t, NewRequest(t, "GET", "/api/v1/version"), http.StatusOK)
	})
}

func TestInstanceNoticeTimeWindow(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	setInstanceNoticeForTest(t, setting.DefaultInstanceNotice())

	now := time.Now().Unix()
	setInstanceNoticeForTest(t, setting.InstanceNotice{
		Enabled:   true,
		Message:   "Future banner",
		Level:     setting.InstanceNoticeLevelInfo,
		StartTime: now + 3600,
		EndTime:   now + 7200,
	})

	resp := MakeRequest(t, NewRequest(t, "GET", "/"), http.StatusOK)
	assert.NotContains(t, resp.Body.String(), "Future banner")

	setInstanceNoticeForTest(t, setting.InstanceNotice{
		Enabled:   true,
		Message:   "Expired banner",
		Level:     setting.InstanceNoticeLevelInfo,
		StartTime: now - 7200,
		EndTime:   now - 3600,
	})

	resp = MakeRequest(t, NewRequest(t, "GET", "/"), http.StatusOK)
	assert.NotContains(t, resp.Body.String(), "Expired banner")
}

func TestInstanceNoticeAdminCRUD(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	setInstanceNoticeForTest(t, setting.DefaultInstanceNotice())

	adminSession := loginUser(t, "user1")
	req := NewRequestWithValues(t, "POST", "/-/admin/config/instance_notice", map[string]string{
		"enabled": "true",
		"message": "Admin set banner",
		"level":   "danger",
	})
	adminSession.MakeRequest(t, req, http.StatusSeeOther)

	notice := setting.GetInstanceNotice(t.Context())
	assert.True(t, notice.Enabled)
	assert.Equal(t, "Admin set banner", notice.Message)
	assert.Equal(t, setting.InstanceNoticeLevelDanger, notice.Level)

	req = NewRequestWithValues(t, "POST", "/-/admin/config/instance_notice", map[string]string{
		"enabled": "true",
		"message": strings.Repeat("a", 2001),
		"level":   "warning",
	})
	adminSession.MakeRequest(t, req, http.StatusSeeOther)

	notice = setting.GetInstanceNotice(t.Context())
	assert.Equal(t, "Admin set banner", notice.Message)
	assert.Equal(t, setting.InstanceNoticeLevelDanger, notice.Level)

	req = NewRequestWithValues(t, "POST", "/-/admin/config/instance_notice", map[string]string{
		"action": "delete",
	})
	adminSession.MakeRequest(t, req, http.StatusSeeOther)

	notice = setting.GetInstanceNotice(t.Context())
	assert.Equal(t, setting.DefaultInstanceNotice(), notice)
}

func setInstanceNoticeForTest(t *testing.T, notice setting.InstanceNotice) {
	t.Helper()
	marshaled, err := json.Marshal(notice)
	require.NoError(t, err)
	require.NoError(t, system_model.SetSettings(t.Context(), map[string]string{
		setting.Config().InstanceNotice.Banner.DynKey(): string(marshaled),
	}))
	config.GetDynGetter().InvalidateCache()
}
