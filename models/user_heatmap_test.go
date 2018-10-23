// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.package models

package models

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetUserHeatmapDataByUser(t *testing.T) {
	// Prepare
	assert.NoError(t, PrepareTestDatabase())

	// Insert some action
	user := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)

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
	assert.Equal(t, len(actions), len(heatmap))
}
