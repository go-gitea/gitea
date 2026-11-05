// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
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

	for _, testCase := range []struct{ name, url string }{
		{"OrgDashboard", "/org/org3/dashboard/-/heatmap"},
		{"OrgTeamDashboard", "/org/org3/dashboard/-/heatmap/team1"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			req := NewRequest(t, "GET", testCase.url)
			resp := session.MakeRequest(t, req, http.StatusOK)

			result := DecodeJSON(t, resp, map[string]any{})
			assert.Contains(t, result, "heatmapData")
			assert.Contains(t, result, "totalContributions")
		})
	}
}
