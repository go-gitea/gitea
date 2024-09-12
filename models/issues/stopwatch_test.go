// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func TestCancelStopwatch(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user1, err := user_model.GetUserByID(db.DefaultContext, 1)
	assert.NoError(t, err)

	issue1, err := issues_model.GetIssueByID(db.DefaultContext, 1)
	assert.NoError(t, err)
	issue2, err := issues_model.GetIssueByID(db.DefaultContext, 2)
	assert.NoError(t, err)

	err = issues_model.CancelStopwatch(db.DefaultContext, user1, issue1)
	assert.NoError(t, err)
	unittest.AssertNotExistsBean(t, &issues_model.Stopwatch{UserID: user1.ID, IssueID: issue1.ID})

	_ = unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{Type: issues_model.CommentTypeCancelTracking, PosterID: user1.ID, IssueID: issue1.ID})

	assert.Nil(t, issues_model.CancelStopwatch(db.DefaultContext, user1, issue2))
}

func TestStopwatchExists(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	assert.True(t, issues_model.StopwatchExists(db.DefaultContext, 1, 1))
	assert.False(t, issues_model.StopwatchExists(db.DefaultContext, 1, 2))
}

func TestHasUserStopwatch(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	exists, sw, _, err := issues_model.HasUserStopwatch(db.DefaultContext, 1)
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, int64(1), sw.ID)

	exists, _, _, err = issues_model.HasUserStopwatch(db.DefaultContext, 3)
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestCreateOrStopIssueStopwatch(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user2, err := user_model.GetUserByID(db.DefaultContext, 2)
	assert.NoError(t, err)
	org3, err := user_model.GetUserByID(db.DefaultContext, 3)
	assert.NoError(t, err)

	issue1, err := issues_model.GetIssueByID(db.DefaultContext, 1)
	assert.NoError(t, err)
	issue2, err := issues_model.GetIssueByID(db.DefaultContext, 2)
	assert.NoError(t, err)

	assert.NoError(t, issues_model.CreateOrStopIssueStopwatch(db.DefaultContext, org3, issue1))
	sw := unittest.AssertExistsAndLoadBean(t, &issues_model.Stopwatch{UserID: 3, IssueID: 1})
	assert.LessOrEqual(t, sw.CreatedUnix, timeutil.TimeStampNow())

	assert.NoError(t, issues_model.CreateOrStopIssueStopwatch(db.DefaultContext, user2, issue2))
	unittest.AssertNotExistsBean(t, &issues_model.Stopwatch{UserID: 2, IssueID: 2})
	unittest.AssertExistsAndLoadBean(t, &issues_model.TrackedTime{UserID: 2, IssueID: 2})
}
