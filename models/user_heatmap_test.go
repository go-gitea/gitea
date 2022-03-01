// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.package models

package models

import (
	"fmt"
	"testing"
	"time"

	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func TestGetUserHeatmapDataByUser(t *testing.T) {
	testCases := []struct {
		desc        string
		userID      int64
		doerID      int64
		CountResult int
		JSONResult  string
	}{
		{
			"self looks at action in private repo",
			2, 2, 1, `[{"timestamp":1603227600,"contributions":1}]`,
		},
		{
			"admin looks at action in private repo",
			2, 1, 1, `[{"timestamp":1603227600,"contributions":1}]`,
		},
		{
			"other user looks at action in private repo",
			2, 3, 0, `[]`,
		},
		{
			"nobody looks at action in private repo",
			2, 0, 0, `[]`,
		},
		{
			"collaborator looks at action in private repo",
			16, 15, 1, `[{"timestamp":1603267200,"contributions":1}]`,
		},
		{
			"no action action not performed by target user",
			3, 3, 0, `[]`,
		},
		{
			"multiple actions performed with two grouped together",
			10, 10, 3, `[{"timestamp":1603009800,"contributions":1},{"timestamp":1603010700,"contributions":2}]`,
		},
	}
	// Prepare
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Mock time
	timeutil.Set(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC))
	defer timeutil.Unset()

	for _, tc := range testCases {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: tc.userID}).(*user_model.User)

		doer := &user_model.User{ID: tc.doerID}
		_, err := unittest.LoadBeanIfExists(doer)
		assert.NoError(t, err)
		if tc.doerID == 0 {
			doer = nil
		}

		// get the action for comparison
		actions, err := GetFeeds(GetFeedsOptions{
			RequestedUser:   user,
			Actor:           doer,
			IncludePrivate:  true,
			OnlyPerformedBy: true,
			IncludeDeleted:  true,
		})
		assert.NoError(t, err)

		// Get the heatmap and compare
		heatmap, err := GetUserHeatmapDataByUser(user, doer)
		var contributions int
		for _, hm := range heatmap {
			contributions += int(hm.Contributions)
		}
		assert.NoError(t, err)
		assert.Len(t, actions, contributions, "invalid action count: did the test data became too old?")
		assert.Equal(t, tc.CountResult, contributions, fmt.Sprintf("testcase '%s'", tc.desc))

		// Test JSON rendering
		jsonData, err := json.Marshal(heatmap)
		assert.NoError(t, err)
		assert.Equal(t, tc.JSONResult, string(jsonData))
	}
}
