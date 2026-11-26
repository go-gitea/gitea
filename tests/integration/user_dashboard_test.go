// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"strings"
	"testing"

	user_model "gitea.dev/models/user"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserDashboardTabs(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user2")

	overviewText := func(t *testing.T, url string) string {
		return NewHTMLParser(t, session.MakeRequest(t, NewRequest(t, "GET", url), http.StatusOK).Body).Find(".dashboard-overview").Text()
	}
	setTab := func(t *testing.T, tab string) {
		session.MakeRequest(t, NewRequestWithJSON(t, "POST", "/user/settings/update_preferences", map[string]string{"dashboardTab": tab}), http.StatusOK)
	}

	assert.Contains(t, overviewText(t, "/"), "pull6")
	assert.Contains(t, overviewText(t, "/org/org3/dashboard/team1"), "pull6")
	assert.Empty(t, overviewText(t, "/?page=1"))
	session.MakeRequest(t, NewRequestWithBody(t, "POST", "/user/settings/update_preferences", strings.NewReader("null")), http.StatusOK)

	setTab(t, "activity")
	setTab(t, "invalid")
	assert.Empty(t, overviewText(t, "/"))
	assert.Empty(t, overviewText(t, "/org/org3/dashboard"))
	showFileTree, err := user_model.GetUserSetting(t.Context(), 2, user_model.SettingsKeyCodeViewShowFileTree, "true")
	require.NoError(t, err)
	assert.Equal(t, "true", showFileTree)

	setTab(t, "overview")
	assert.NotEmpty(t, overviewText(t, "/"))
}
