// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activities_test

import (
	"testing"
	"time"

	activities_model "gitea.dev/models/activities"
	"gitea.dev/models/db"
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

func TestCountHiddenUserActivities(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// mock time so the heatmap window covers the fixture actions
	timeutil.MockSet(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC))
	defer timeutil.MockUnset()

	countForPage := func(t *testing.T, user, doer *user_model.User, date string, page, pageSize int) int64 {
		opts := activities_model.GetFeedsOptions{
			RequestedUser:   user,
			Actor:           doer,
			IncludePrivate:  false,
			OnlyPerformedBy: true,
			Date:            date,
			ListOptions:     db.ListOptions{Page: page, PageSize: pageSize},
		}
		items, _, err := activities_model.GetFeeds(t.Context(), opts)
		assert.NoError(t, err)
		hidden, err := activities_model.CountHiddenUserActivities(t.Context(), opts, items)
		assert.NoError(t, err)
		return hidden
	}

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	user16 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 16})
	user15 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 15})

	// anonymous viewer, day filter: the single private action is hidden
	assert.EqualValues(t, 1, countForPage(t, user2, nil, "2020-10-20", 1, 20))
	// day without actions
	assert.EqualValues(t, 0, countForPage(t, user2, nil, "2020-10-19", 1, 20))
	// no date filter: the whole feed rolls up on the (only) page
	assert.EqualValues(t, 1, countForPage(t, user2, nil, "", 1, 20))
	// an invalid date behaves like the unfiltered feed, matching FeedDateCond
	assert.EqualValues(t, 1, countForPage(t, user2, nil, "not-a-date", 1, 20))
	// pages beyond the visible feed have an empty span
	assert.EqualValues(t, 0, countForPage(t, user2, nil, "", 3, 20))

	// a collaborator's visible profile feed hides private actions too, so the
	// hidden count must equal the heatmap count for that day
	user16.ShowPrivateActivity = true
	heatmap, err := activities_model.GetUserHeatmapDataByUser(t.Context(), user16, user15)
	assert.NoError(t, err)
	var contributions int64
	for _, hm := range heatmap {
		contributions += hm.Contributions
	}
	hidden := countForPage(t, user16, user15, "2020-10-21", 1, 20)
	assert.EqualValues(t, 1, hidden)
	assert.Equal(t, contributions, hidden, "heatmap(day) == visible(day) + hidden(day)")

	// page spans must partition the timeline: interleave extra public and private
	// actions newer than user2's fixture action (ts 1603228283 in private repo2)
	for i, action := range []*activities_model.Action{
		{UserID: 2, ActUserID: 2, OpType: activities_model.ActionCreateIssue, RepoID: 1, IsPrivate: false}, // public, oldest
		{UserID: 2, ActUserID: 2, OpType: activities_model.ActionCreateIssue, RepoID: 2, IsPrivate: true},  // hidden
		{UserID: 2, ActUserID: 2, OpType: activities_model.ActionCreateIssue, RepoID: 1, IsPrivate: false}, // public
		{UserID: 2, ActUserID: 2, OpType: activities_model.ActionCloseIssue, RepoID: 1, IsPrivate: false},  // public, newest
	} {
		assert.NoError(t, db.Insert(t.Context(), action))
		// bypass the xorm `created` tag which overrides provided timestamps
		_, err := db.GetEngine(t.Context()).Exec("UPDATE action SET created_unix=? WHERE id=?",
			1603300000+int64(i)*1000, action.ID)
		assert.NoError(t, err)
	}

	// visible to anonymous: the 3 public actions; hidden: fixture action 1 + 1 inserted
	perPage := make([]int64, 0, 4)
	var sum int64
	for page := 1; page <= 4; page++ {
		hidden := countForPage(t, user2, nil, "", page, 1)
		perPage = append(perPage, hidden)
		sum += hidden
	}
	// a hidden action between two visible ones lands on the older neighbour's page;
	// hidden actions older than all visible ones land on the last page
	assert.Equal(t, []int64{0, 0, 1, 1}, perPage)
	assert.EqualValues(t, 2, sum, "per-page hidden counts must sum to the feed-wide total")
}
