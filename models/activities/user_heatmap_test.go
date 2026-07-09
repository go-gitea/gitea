// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activities_test

import (
	"testing"
	"time"

	activities_model "gitea.dev/models/activities"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/json"
	"gitea.dev/modules/structs"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func TestGetUserHeatmapDataByUser(t *testing.T) {
	testCases := []struct {
		desc                string
		userID              int64
		doerID              int64
		showPrivateActivity bool
		CountResult         int
		JSONResult          string
	}{
		{
			"self looks at action in private repo",
			2, 2, false, 1, `[{"timestamp":1603227600,"contributions":1}]`,
		},
		{
			"admin looks at action in private repo",
			2, 1, false, 1, `[{"timestamp":1603227600,"contributions":1}]`,
		},
		{
			"other user looks at action in private repo",
			2, 3, false, 0, `[]`,
		},
		{
			"nobody looks at action in private repo",
			2, 0, false, 0, `[]`,
		},
		{
			"collaborator looks at action in private repo",
			16, 15, false, 1, `[{"timestamp":1603267200,"contributions":1}]`,
		},
		{
			"no action action not performed by target user",
			3, 3, false, 0, `[]`,
		},
		{
			"multiple actions performed with two grouped together",
			10, 10, false, 3, `[{"timestamp":1603009800,"contributions":1},{"timestamp":1603010700,"contributions":2}]`,
		},
		{
			"nobody looks at private repo action, owner shows private activity",
			2, 0, true, 1, `[{"timestamp":1603227600,"contributions":1}]`,
		},
		{
			"other user looks at private repo action, owner shows private activity",
			2, 3, true, 1, `[{"timestamp":1603227600,"contributions":1}]`,
		},
		{
			"self looks at private repo action, owner shows private activity",
			2, 2, true, 1, `[{"timestamp":1603227600,"contributions":1}]`,
		},
		{
			"collaborator looks at private repo action, owner shows private activity",
			16, 15, true, 1, `[{"timestamp":1603267200,"contributions":1}]`,
		},
		{
			"nobody looks at multiple actions, owner shows private activity",
			10, 0, true, 3, `[{"timestamp":1603009800,"contributions":1},{"timestamp":1603010700,"contributions":2}]`,
		},
	}
	// Prepare
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Mock time
	timeutil.MockSet(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC))
	defer timeutil.MockUnset()

	for _, tc := range testCases {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: tc.userID})
		user.ShowPrivateActivity = tc.showPrivateActivity

		var doer *user_model.User
		if tc.doerID != 0 {
			doer = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: tc.doerID})
		}

		// when private activity is shown the heatmap must match the owner's own view
		feedsActor := doer
		if tc.showPrivateActivity {
			feedsActor = user
		}

		// get the action for comparison
		actions, count, err := activities_model.GetFeeds(t.Context(), activities_model.GetFeedsOptions{
			RequestedUser:   user,
			Actor:           feedsActor,
			IncludePrivate:  true,
			OnlyPerformedBy: true,
			IncludeDeleted:  true,
		})
		assert.NoError(t, err)

		// Get the heatmap and compare
		heatmap, err := activities_model.GetUserHeatmapDataByUser(t.Context(), user, doer)
		var contributions int
		for _, hm := range heatmap {
			contributions += int(hm.Contributions)
		}
		assert.NoError(t, err)
		assert.Len(t, actions, contributions, "invalid action count: did the test data became too old?")
		assert.Equal(t, count, int64(contributions))
		assert.Equal(t, tc.CountResult, contributions, "testcase '%s'", tc.desc)

		// Test JSON rendering
		jsonData, err := json.Marshal(heatmap)
		assert.NoError(t, err)
		assert.JSONEq(t, tc.JSONResult, string(jsonData))
	}
}

func TestGetUserHeatmapDataByUserHiddenFromViewer(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	timeutil.MockSet(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC))
	defer timeutil.MockUnset()

	// a non-public user opting in must stay hidden from viewers who cannot see
	// the profile at all, instead of exposing counts via the owner fast path
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	user.ShowPrivateActivity = true
	user.Visibility = structs.VisibleTypePrivate

	heatmap, err := activities_model.GetUserHeatmapDataByUser(t.Context(), user, nil)
	assert.NoError(t, err)
	assert.Empty(t, heatmap)
}

func TestCountUserActivitiesOnDate(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// mock time so the heatmap window covers the fixture actions
	timeutil.MockSet(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC))
	defer timeutil.MockUnset()

	testCases := []struct {
		desc   string
		userID int64
		date   string
		count  int64
	}{
		{"private repo action counted", 2, "2020-10-20", 1},
		{"day without actions", 2, "2020-10-19", 0},
		{"private repo action of another user", 16, "2020-10-21", 1},
		{"multiple actions on one day", 10, "2020-10-18", 3},
	}
	for _, tc := range testCases {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: tc.userID})
		count, err := activities_model.CountUserActivitiesOnDate(t.Context(), user, tc.date)
		assert.NoError(t, err)
		assert.Equal(t, tc.count, count, "testcase '%s'", tc.desc)
	}

	// unparseable dates must error instead of counting all time
	for _, invalidDate := range []string{"", "not-a-date"} {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		_, err := activities_model.CountUserActivitiesOnDate(t.Context(), user, invalidDate)
		assert.Error(t, err, "date %q should be rejected", invalidDate)
	}

	// the placeholder subtraction scenario: a collaborator's visible profile feed
	// hides private actions, so hidden = total - visible must equal the heatmap count
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 16})
	user.ShowPrivateActivity = true
	viewer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 15})
	_, visible, err := activities_model.GetFeeds(t.Context(), activities_model.GetFeedsOptions{
		RequestedUser:   user,
		Actor:           viewer,
		IncludePrivate:  false,
		OnlyPerformedBy: true,
		Date:            "2020-10-21",
	})
	assert.NoError(t, err)
	total, err := activities_model.CountUserActivitiesOnDate(t.Context(), user, "2020-10-21")
	assert.NoError(t, err)
	heatmap, err := activities_model.GetUserHeatmapDataByUser(t.Context(), user, viewer)
	assert.NoError(t, err)
	var contributions int64
	for _, hm := range heatmap {
		contributions += hm.Contributions
	}
	assert.EqualValues(t, 0, visible)
	assert.Equal(t, int64(1), total-visible)
	assert.Equal(t, contributions, visible+(total-visible), "heatmap(day) == visible(day) + hidden(day)")
}
