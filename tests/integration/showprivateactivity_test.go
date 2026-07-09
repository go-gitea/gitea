// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	activities_model "gitea.dev/models/activities"
	"gitea.dev/modules/timeutil"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

// user2 owns the private repo2 with one action on 2020-10-20 (fixture action 1)
const (
	showPrivateActivityTestUser = "user2"
	showPrivateActivityTestDate = "2020-10-20"
	privateContributionsText    = "contribution in private repositories"
)

func testShowPrivateActivityHelperEnableSetting(t *testing.T, session *TestSession, keepActivityPrivate bool) {
	settings := map[string]string{
		"name":                  showPrivateActivityTestUser,
		"email":                 showPrivateActivityTestUser + "@example.com",
		"language":              "en-US",
		"show_private_activity": "1",
	}
	if keepActivityPrivate {
		settings["keep_activity_private"] = "1"
	}
	req := NewRequestWithValues(t, "POST", "/user/settings", settings)
	session.MakeRequest(t, req, http.StatusSeeOther)
}

func testShowPrivateActivityHelperTotalContributionsFromAPI(t *testing.T) int64 {
	req := NewRequestf(t, "GET", "/api/v1/users/%s/heatmap", showPrivateActivityTestUser)
	resp := MakeRequest(t, req, http.StatusOK)

	items := DecodeJSON(t, resp, []*activities_model.UserHeatmapData{})

	var total int64
	for _, item := range items {
		total += item.Contributions
	}
	return total
}

func testShowPrivateActivityHelperTotalContributionsFromWeb(t *testing.T) int64 {
	req := NewRequestf(t, "GET", "/%s/-/heatmap", showPrivateActivityTestUser)
	resp := MakeRequest(t, req, http.StatusOK)

	result := DecodeJSON(t, resp, map[string]any{})
	total, _ := result["totalContributions"].(float64)
	return int64(total)
}

func testShowPrivateActivityHelperGetDayFeed(t *testing.T, session *TestSession) *HTMLDoc {
	req := NewRequestf(t, "GET", "/%s?tab=activity&date=%s", showPrivateActivityTestUser, showPrivateActivityTestDate)
	var resp *httptest.ResponseRecorder
	if session != nil {
		resp = session.MakeRequest(t, req, http.StatusOK)
	} else {
		resp = MakeRequest(t, req, http.StatusOK)
	}
	return NewHTMLParser(t, resp.Body)
}

func TestShowPrivateActivity(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	// mock time so the fixture actions fall within the heatmap's time window
	timeutil.MockSet(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC))
	defer timeutil.MockUnset()

	session := loginUser(t, showPrivateActivityTestUser)

	t.Run("SettingOffHidesPrivateContributions", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		assert.EqualValues(t, 0, testShowPrivateActivityHelperTotalContributionsFromAPI(t))
		assert.EqualValues(t, 0, testShowPrivateActivityHelperTotalContributionsFromWeb(t))

		htmlDoc := testShowPrivateActivityHelperGetDayFeed(t, nil)
		assert.NotContains(t, htmlDoc.doc.Find("#activity-feed").Text(), privateContributionsText)
	})

	t.Run("SettingOnCountsPrivateContributions", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		testShowPrivateActivityHelperEnableSetting(t, session, false)

		assert.EqualValues(t, 1, testShowPrivateActivityHelperTotalContributionsFromAPI(t))
		assert.EqualValues(t, 1, testShowPrivateActivityHelperTotalContributionsFromWeb(t))
	})

	t.Run("SettingOnShowsPlaceholderWithoutDetails", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		testShowPrivateActivityHelperEnableSetting(t, session, false)

		htmlDoc := testShowPrivateActivityHelperGetDayFeed(t, nil)
		feed := htmlDoc.doc.Find("#activity-feed")
		assert.Contains(t, feed.Text(), "1 "+privateContributionsText)
		// no details of the private repo may leak
		assert.Zero(t, feed.Find("a[href*='/user2/repo2']").Length())
	})

	t.Run("NoPlaceholderWithoutValidDate", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		testShowPrivateActivityHelperEnableSetting(t, session, false)

		for _, uri := range []string{
			"/" + showPrivateActivityTestUser + "?tab=activity",                 // general feed
			"/" + showPrivateActivityTestUser + "?tab=activity&date=not-a-date", // invalid date must not count all time
		} {
			req := NewRequest(t, "GET", uri)
			resp := MakeRequest(t, req, http.StatusOK)
			htmlDoc := NewHTMLParser(t, resp.Body)
			assert.NotContains(t, htmlDoc.doc.Find("#activity-feed").Text(), privateContributionsText, "no placeholder expected for %s", uri)
		}
	})

	t.Run("SelfSeesRealActionsInsteadOfPlaceholder", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		testShowPrivateActivityHelperEnableSetting(t, session, false)

		htmlDoc := testShowPrivateActivityHelperGetDayFeed(t, session)
		feed := htmlDoc.doc.Find("#activity-feed")
		assert.NotContains(t, feed.Text(), privateContributionsText)
		assert.Positive(t, feed.Find(".item").Length())
	})

	t.Run("KeepActivityPrivateWins", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		testShowPrivateActivityHelperEnableSetting(t, session, true)

		assert.EqualValues(t, 0, testShowPrivateActivityHelperTotalContributionsFromAPI(t))
		assert.EqualValues(t, 0, testShowPrivateActivityHelperTotalContributionsFromWeb(t))

		htmlDoc := testShowPrivateActivityHelperGetDayFeed(t, nil)
		feed := htmlDoc.doc.Find("#activity-feed")
		assert.NotContains(t, feed.Text(), privateContributionsText)
		assert.Zero(t, feed.Find(".item").Length())
	})
}
