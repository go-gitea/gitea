// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/base"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type NotificationBefore331 struct {
	ID     int64 `xorm:"pk autoincr"`
	UserID int64 `xorm:"NOT NULL"`
	RepoID int64 `xorm:"NOT NULL"`

	Status uint8 `xorm:"SMALLINT NOT NULL"`
	Source uint8 `xorm:"SMALLINT NOT NULL"`

	IssueID   int64 `xorm:"NOT NULL"`
	CommitID  string
	CommentID int64
	ReleaseID int64

	UpdatedBy int64 `xorm:"NOT NULL"`

	CreatedUnix timeutil.TimeStamp `xorm:"created NOT NULL"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated NOT NULL"`
}

func (NotificationBefore331) TableName() string {
	return "notification"
}

func TestAddReleaseNotificationBackfillsNotificationDedupe(t *testing.T) {
	x, deferable := base.PrepareTestEnv(t, 0, new(NotificationBefore331))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	testData := []*NotificationBefore331{
		{UserID: 1, RepoID: 1, Status: 1, Source: 1, IssueID: 42, UpdatedBy: 2},
		{UserID: 1, RepoID: 1, Status: 1, Source: 2, IssueID: 43, UpdatedBy: 2},
		{UserID: 1, RepoID: 2, Status: 1, Source: 3, CommitID: "abc123", UpdatedBy: 2},
		{UserID: 1, RepoID: 3, Status: 1, Source: 5, ReleaseID: 7, UpdatedBy: 2},
		{UserID: 1, RepoID: 4, Status: 1, Source: 4, UpdatedBy: 2},
	}
	for _, data := range testData {
		_, err := x.Insert(data)
		require.NoError(t, err)
	}

	require.NoError(t, AddReleaseNotification(x))

	var notifications []*NotificationV331
	require.NoError(t, x.Table("notification").Asc("id").Find(&notifications))
	require.Len(t, notifications, len(testData))

	require.NotNil(t, notifications[0].UniqueKey)
	assert.Equal(t, "issue-42", *notifications[0].UniqueKey)

	require.NotNil(t, notifications[1].UniqueKey)
	assert.Equal(t, "pull-43", *notifications[1].UniqueKey)

	require.NotNil(t, notifications[2].UniqueKey)
	assert.Equal(t, "commit-2-abc123", *notifications[2].UniqueKey)

	require.NotNil(t, notifications[3].UniqueKey)
	assert.Equal(t, "release-7", *notifications[3].UniqueKey)

	assert.Nil(t, notifications[4].UniqueKey)
}

func TestAddReleaseNotificationDeduplicatesLegacyNotificationRows(t *testing.T) {
	x, deferable := base.PrepareTestEnv(t, 0, new(NotificationBefore331))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	testData := []*NotificationBefore331{
		{UserID: 1, RepoID: 2, Status: 2, Source: 3, CommitID: "abc123", UpdatedBy: 2, UpdatedUnix: 100},
		{UserID: 1, RepoID: 2, Status: 1, Source: 3, CommitID: "abc123", UpdatedBy: 3, UpdatedUnix: 200},
		{UserID: 1, RepoID: 2, Status: 3, Source: 3, CommitID: "abc123", UpdatedBy: 4, UpdatedUnix: 150},
	}
	for _, data := range testData {
		_, err := x.Insert(data)
		require.NoError(t, err)
	}

	existingNotifications := make([]*NotificationBefore331, 0, len(testData))
	require.NoError(t, x.Table("notification").Desc("updated_unix", "id").Find(&existingNotifications))
	require.NotEmpty(t, existingNotifications)
	expectedKeeper := existingNotifications[0]

	require.NoError(t, AddReleaseNotification(x))

	var notifications []*NotificationV331
	require.NoError(t, x.Table("notification").Find(&notifications))
	require.Len(t, notifications, 1)

	require.NotNil(t, notifications[0].UniqueKey)
	assert.Equal(t, "commit-2-abc123", *notifications[0].UniqueKey)
	assert.Equal(t, notificationStatusPinnedV331, notifications[0].Status)
	assert.Equal(t, expectedKeeper.UpdatedBy, notifications[0].UpdatedBy)
	assert.Equal(t, expectedKeeper.UpdatedUnix, notifications[0].UpdatedUnix)
}
