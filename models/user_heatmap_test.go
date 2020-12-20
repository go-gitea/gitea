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
		doerID      int64
		CountResult int
		JSONResult  string
	}{
		{2, 2, 1, `[{"timestamp":1603152000,"contributions":1}]`},
		{2, 0, 0, `[]`},
		{3, 3, 0, `[]`},
	}
	// Prepare
	assert.NoError(t, PrepareTestDatabase())

	for _, tc := range testCases {
		user := AssertExistsAndLoadBean(t, &User{ID: tc.userID}).(*User)
		doer := &User{ID: tc.doerID}
		_, err := loadBeanIfExists(doer)
		assert.NoError(t, err)

		// get the action for comparison
		actions, err := GetFeeds(GetFeedsOptions{
			RequestedUser:   user,
			Actor:           doer,
			IncludePrivate:  true,
			OnlyPerformedBy: false,
			IncludeDeleted:  true,
		})
		assert.NoError(t, err)

		// Get the heatmap and compare
		heatmap, err := GetUserHeatmapDataByUser(user, doer)
		assert.NoError(t, err)
		assert.Equal(t, len(actions), len(heatmap), "invalid action count: did the test data became too old?")
		assert.Equal(t, tc.CountResult, len(heatmap))

		//Test JSON rendering
		jsonData, err := json.Marshal(heatmap)
		assert.NoError(t, err)
		assert.Equal(t, tc.JSONResult, string(jsonData))
	}
}
