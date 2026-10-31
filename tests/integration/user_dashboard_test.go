// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestUserDashboardTabs(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user2")

	getDashboard := func(t *testing.T, url string) *HTMLDoc {
		resp := session.MakeRequest(t, NewRequest(t, "GET", url), http.StatusOK)
		return NewHTMLParser(t, resp.Body)
	}
	isOverview := func(t *testing.T, url string) bool {
		return getDashboard(t, url).Find(".dashboard-overview").Length() > 0
	}

	assert.False(t, isOverview(t, "/"))
	assert.True(t, isOverview(t, "/org/org3/dashboard?tab=overview"))
	assert.True(t, isOverview(t, "/org/org3/dashboard/team1?tab=overview"))

	session.MakeRequest(t, NewRequestWithValues(t, "POST", "/user/settings/appearance/dashboard_tab", map[string]string{
		"dashboard_tab": "overview",
	}), http.StatusSeeOther)
	assert.False(t, isOverview(t, "/?tab=activity"))
	assert.False(t, isOverview(t, "/?date=2024-01-01"))

	doc := getDashboard(t, "/")
	assert.Contains(t, doc.Find(".dashboard-overview-list").Text(), "pull6")
}
