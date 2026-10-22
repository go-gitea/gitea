// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"gitea.dev/modules/timeutil"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestHeatmapEndpoints(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// Mock time so fixture actions fall within the heatmap's time window
	timeutil.MockSet(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC))
	defer timeutil.MockUnset()

	session := loginUser(t, "user2")

	t.Run("UserProfile", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		req := NewRequest(t, "GET", "/user2/-/heatmap")
		resp := session.MakeRequest(t, req, http.StatusOK)

		result := DecodeJSON(t, resp, map[string]any{})
		assert.Contains(t, result, "heatmapData")
		assert.Contains(t, result, "totalContributions")
		assert.Positive(t, result["totalContributions"])
	})

	t.Run("PrivateUserProfileAnonymous", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		req := NewRequest(t, "GET", "/user31/-/heatmap")
		MakeRequest(t, req, http.StatusNotFound)
	})

	t.Run("OrgDashboard", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		req := NewRequest(t, "GET", "/org/org3/dashboard/-/heatmap")
		resp := session.MakeRequest(t, req, http.StatusOK)

		result := DecodeJSON(t, resp, map[string]any{})
		assert.Contains(t, result, "heatmapData")
		assert.Contains(t, result, "totalContributions")
	})

	t.Run("OrgTeamDashboard", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		req := NewRequest(t, "GET", "/org/org3/dashboard/-/heatmap/team1")
		resp := session.MakeRequest(t, req, http.StatusOK)

		result := DecodeJSON(t, resp, map[string]any{})
		assert.Contains(t, result, "heatmapData")
		assert.Contains(t, result, "totalContributions")
	})

	t.Run("FeedDivOnly", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		for link, hasItems := range map[string]bool{
			"/?div-only=true": true,
			"/user2?tab=activity&div-only=true&year=2020": true,
			"/user2?tab=activity&div-only=true&year=2019": false,
		} {
			resp := session.MakeRequest(t, NewRequest(t, "GET", link), http.StatusOK)
			assert.True(t, strings.HasPrefix(resp.Body.String(), `<div id="activity-feed"`), link)
			assert.Equal(t, hasItems, NewHTMLParser(t, resp.Body).Find("#activity-feed > .item").Length() > 0, link)
		}
	})
}
