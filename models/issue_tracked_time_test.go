// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAddTime(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	user3, err := GetUserByID(3)
	assert.NoError(t, err)

	issue1, err := GetIssueByID(1)
	assert.NoError(t, err)

	//3661 = 1h 1min 1s
	trackedTime, err := AddTime(user3, issue1, 3661, time.Now())
	assert.NoError(t, err)
	assert.Equal(t, int64(3), trackedTime.UserID)
	assert.Equal(t, int64(1), trackedTime.IssueID)
	assert.Equal(t, int64(3661), trackedTime.Time)

	tt := AssertExistsAndLoadBean(t, &TrackedTime{UserID: 3, IssueID: 1}).(*TrackedTime)
	assert.Equal(t, int64(3661), tt.Time)

	comment := AssertExistsAndLoadBean(t, &Comment{Type: CommentTypeAddTimeManual, PosterID: 3, IssueID: 1}).(*Comment)
	assert.Equal(t, comment.Content, "1h 1min 1s")
}

func TestGetTrackedTimes(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	// by Issue
	times, err := GetTrackedTimes(FindTrackedTimesOptions{IssueID: 1})
	assert.NoError(t, err)
	assert.Len(t, times, 1)
	assert.Equal(t, int64(400), times[0].Time)

	times, err = GetTrackedTimes(FindTrackedTimesOptions{IssueID: -1})
	assert.NoError(t, err)
	assert.Len(t, times, 0)

	// by User
	times, err = GetTrackedTimes(FindTrackedTimesOptions{UserID: 1})
	assert.NoError(t, err)
	assert.Len(t, times, 3)
	assert.Equal(t, int64(400), times[0].Time)

	times, err = GetTrackedTimes(FindTrackedTimesOptions{UserID: 3})
	assert.NoError(t, err)
	assert.Len(t, times, 0)

	// by Repo
	times, err = GetTrackedTimes(FindTrackedTimesOptions{RepositoryID: 2})
	assert.NoError(t, err)
	assert.Len(t, times, 3)
	assert.Equal(t, int64(1), times[0].Time)
	issue, err := GetIssueByID(times[0].IssueID)
	assert.NoError(t, err)
	assert.Equal(t, issue.RepoID, int64(2))

	times, err = GetTrackedTimes(FindTrackedTimesOptions{RepositoryID: 1})
	assert.NoError(t, err)
	assert.Len(t, times, 5)

	times, err = GetTrackedTimes(FindTrackedTimesOptions{RepositoryID: 10})
	assert.NoError(t, err)
	assert.Len(t, times, 0)
}

func TestTotalTimes(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	total, err := TotalTimes(FindTrackedTimesOptions{IssueID: 1})
	assert.NoError(t, err)
	assert.Len(t, total, 1)
	for user, time := range total {
		assert.Equal(t, int64(1), user.ID)
		assert.Equal(t, "6min 40s", time)
	}

	total, err = TotalTimes(FindTrackedTimesOptions{IssueID: 2})
	assert.NoError(t, err)
	assert.Len(t, total, 2)
	for user, time := range total {
		if user.ID == 2 {
			assert.Equal(t, "1h 1min 2s", time)
		} else if user.ID == 1 {
			assert.Equal(t, "20s", time)
		} else {
			assert.Error(t, assert.AnError)
		}
	}

	total, err = TotalTimes(FindTrackedTimesOptions{IssueID: 5})
	assert.NoError(t, err)
	assert.Len(t, total, 1)
	for user, time := range total {
		assert.Equal(t, int64(2), user.ID)
		assert.Equal(t, "1s", time)
	}

	total, err = TotalTimes(FindTrackedTimesOptions{IssueID: 4})
	assert.NoError(t, err)
	assert.Len(t, total, 2)
}
