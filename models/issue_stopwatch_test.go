// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func TestCancelStopwatch(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user1, err := user_model.GetUserByID(1)
	assert.NoError(t, err)

	issue1, err := GetIssueByID(1)
	assert.NoError(t, err)
	issue2, err := GetIssueByID(2)
	assert.NoError(t, err)

	err = CancelStopwatch(user1, issue1)
	assert.NoError(t, err)
	unittest.AssertNotExistsBean(t, &Stopwatch{UserID: user1.ID, IssueID: issue1.ID})

	_ = unittest.AssertExistsAndLoadBean(t, &Comment{Type: CommentTypeCancelTracking, PosterID: user1.ID, IssueID: issue1.ID})

	assert.Nil(t, CancelStopwatch(user1, issue2))
}

func TestStopwatchExists(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	assert.True(t, StopwatchExists(1, 1))
	assert.False(t, StopwatchExists(1, 2))
}

func TestHasUserStopwatch(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	exists, sw, err := HasUserStopwatch(1)
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, int64(1), sw.ID)

	exists, _, err = HasUserStopwatch(3)
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestCreateOrStopIssueStopwatch(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user2, err := user_model.GetUserByID(2)
	assert.NoError(t, err)
	user3, err := user_model.GetUserByID(3)
	assert.NoError(t, err)

	issue1, err := GetIssueByID(1)
	assert.NoError(t, err)
	issue2, err := GetIssueByID(2)
	assert.NoError(t, err)

	assert.NoError(t, CreateOrStopIssueStopwatch(user3, issue1))
	sw := unittest.AssertExistsAndLoadBean(t, &Stopwatch{UserID: 3, IssueID: 1}).(*Stopwatch)
	assert.LessOrEqual(t, sw.CreatedUnix, timeutil.TimeStampNow())

	assert.NoError(t, CreateOrStopIssueStopwatch(user2, issue2))
	unittest.AssertNotExistsBean(t, &Stopwatch{UserID: 2, IssueID: 2})
	unittest.AssertExistsAndLoadBean(t, &TrackedTime{UserID: 2, IssueID: 2})
}
