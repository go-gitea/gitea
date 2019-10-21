// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.package models

package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetUserHeatmapDataByUser(t *testing.T) {
	testCases := []struct {
		userID      int64
		CountResult int
		JSONResult  string
	}{
		{2, 1, `[{"timestamp":1571616000,"contributions":1}]`},
		{3, 0, `[]`},
	}
	// Prepare
	assert.NoError(t, PrepareTestDatabase())

	for _, tc := range testCases {

		// Insert some action
		user := AssertExistsAndLoadBean(t, &User{ID: tc.userID}).(*User)

		// get the action for comparison
		actions, err := GetFeeds(GetFeedsOptions{
			RequestedUser:    user,
			RequestingUserID: user.ID,
			IncludePrivate:   true,
			OnlyPerformedBy:  false,
			IncludeDeleted:   true,
		})
		assert.NoError(t, err)

		// Get the heatmap and compare
		heatmap, err := GetUserHeatmapDataByUser(user)
		assert.NoError(t, err)
		assert.Equal(t, len(actions), len(heatmap), "invalid action count: did the test data became too old?")
		assert.Equal(t, tc.CountResult, len(heatmap))

		//Test JSON rendering
		jsonData, err := json.Marshal(heatmap)
		assert.NoError(t, err)
		assert.Equal(t, tc.JSONResult, string(jsonData))
	}
}
