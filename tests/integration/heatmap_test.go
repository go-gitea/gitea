// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/tests"

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

		var result map[string]any
		DecodeJSON(t, resp, &result)
		assert.Contains(t, result, "heatmapData")
		assert.Contains(t, result, "totalContributions")
		assert.Positive(t, result["totalContributions"])
	})

	t.Run("OrgDashboard", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		req := NewRequest(t, "GET", "/org/org3/dashboard/-/heatmap")
		resp := session.MakeRequest(t, req, http.StatusOK)

		var result map[string]any
		DecodeJSON(t, resp, &result)
		assert.Contains(t, result, "heatmapData")
		assert.Contains(t, result, "totalContributions")
	})

	t.Run("OrgTeamDashboard", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		req := NewRequest(t, "GET", "/org/org3/dashboard/-/heatmap/team1")
		resp := session.MakeRequest(t, req, http.StatusOK)

		var result map[string]any
		DecodeJSON(t, resp, &result)
		assert.Contains(t, result, "heatmapData")
		assert.Contains(t, result, "totalContributions")
	})
}
