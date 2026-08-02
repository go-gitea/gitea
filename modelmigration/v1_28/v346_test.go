// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_28

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// notificationBeforeV346 is the notification table as it looked before the subject
// identity columns were introduced.
type notificationBeforeV346 struct {
	ID     int64 `xorm:"pk autoincr"`
	UserID int64 `xorm:"NOT NULL"`
	RepoID int64 `xorm:"NOT NULL"`

	Status uint8 `xorm:"SMALLINT NOT NULL"`
	Source uint8 `xorm:"SMALLINT NOT NULL"`

	IssueID   int64 `xorm:"NOT NULL"`
	CommitID  string
	CommentID int64

	UpdatedBy int64 `xorm:"NOT NULL"`

	CreatedUnix timeutil.TimeStamp `xorm:"created NOT NULL"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated NOT NULL"`
}

func (notificationBeforeV346) TableName() string {
	return "notification"
}

func TestAddNotificationSubjectIdentityBackfillsEachSource(t *testing.T) {
	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(notificationBeforeV346))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	testData := []*notificationBeforeV346{
		{UserID: 1, RepoID: 1, Status: 1, Source: notificationSourceIssueV346, IssueID: 42, UpdatedBy: 2},
		{UserID: 1, RepoID: 1, Status: 1, Source: notificationSourcePullRequestV346, IssueID: 43, UpdatedBy: 2},
		{UserID: 1, RepoID: 2, Status: 1, Source: notificationSourceCommitV346, CommitID: "abc123", UpdatedBy: 2},
		{UserID: 1, RepoID: 4, Status: 1, Source: notificationSourceRepositoryV346, UpdatedBy: 2},
	}
	for _, data := range testData {
		_, err := x.Insert(data)
		require.NoError(t, err)
	}

	require.NoError(t, AddNotificationSubjectIdentity(x))

	var notifications []*NotificationV346
	require.NoError(t, x.Table("notification").Asc("id").Find(&notifications))
	require.Len(t, notifications, len(testData))

	assert.Equal(t, int64(42), notifications[0].SubjectID, "issue keeps its issue id")
	assert.Empty(t, notifications[0].SubjectRef)

	assert.Equal(t, int64(43), notifications[1].SubjectID, "pull request keeps its issue id")

	assert.Equal(t, "abc123", notifications[2].SubjectRef, "commit is identified by its sha")
	assert.Equal(t, int64(0), notifications[2].SubjectID)

	assert.Equal(t, int64(0), notifications[3].SubjectID, "repository needs no subject, repo_id identifies it")
	assert.Equal(t, int64(4), notifications[3].RepoID)
}

// Two repository notifications for different repos must stay distinct: they carry no
// subject at all, so only repo_id being part of the unique index keeps them apart.
func TestAddNotificationSubjectIdentityKeepsRepositoriesDistinct(t *testing.T) {
	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(notificationBeforeV346))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	for _, repoID := range []int64{7, 8} {
		_, err := x.Insert(&notificationBeforeV346{
			UserID: 1, RepoID: repoID, Status: 1, Source: notificationSourceRepositoryV346, UpdatedBy: 2,
		})
		require.NoError(t, err)
	}

	require.NoError(t, AddNotificationSubjectIdentity(x))

	var notifications []*NotificationV346
	require.NoError(t, x.Table("notification").Asc("id").Find(&notifications))
	require.Len(t, notifications, 2)
	assert.Equal(t, int64(7), notifications[0].RepoID)
	assert.Equal(t, int64(8), notifications[1].RepoID)
}

func TestAddNotificationSubjectIdentityDedupesAndKeepsPinned(t *testing.T) {
	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(notificationBeforeV346))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	// Three rows for the same commit: read, unread, pinned. Only one may survive, and it
	// must keep the pinned status so nothing the user marked is lost.
	testData := []*notificationBeforeV346{
		{UserID: 1, RepoID: 2, Status: 2, Source: notificationSourceCommitV346, CommitID: "abc123", UpdatedBy: 2, UpdatedUnix: 100},
		{UserID: 1, RepoID: 2, Status: 1, Source: notificationSourceCommitV346, CommitID: "abc123", UpdatedBy: 3, UpdatedUnix: 200},
		{UserID: 1, RepoID: 2, Status: 3, Source: notificationSourceCommitV346, CommitID: "abc123", UpdatedBy: 4, UpdatedUnix: 150},
	}
	for _, data := range testData {
		_, err := x.Insert(data)
		require.NoError(t, err)
	}

	var existing []*notificationBeforeV346
	require.NoError(t, x.Table("notification").Desc("updated_unix", "id").Find(&existing))
	require.NotEmpty(t, existing)
	expectedKeeper := existing[0]

	require.NoError(t, AddNotificationSubjectIdentity(x))

	var notifications []*NotificationV346
	require.NoError(t, x.Table("notification").Find(&notifications))
	require.Len(t, notifications, 1)

	assert.Equal(t, "abc123", notifications[0].SubjectRef)
	assert.Equal(t, uint8(notificationStatusPinnedV346), notifications[0].Status, "pinned must win over unread and read")
	assert.Equal(t, expectedKeeper.UpdatedBy, notifications[0].UpdatedBy, "the most recently updated row survives")
}

// A row with a source this migration does not know about must still survive rather than
// aborting the whole upgrade.
func TestAddNotificationSubjectIdentityToleratesUnknownSource(t *testing.T) {
	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(notificationBeforeV346))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	_, err := x.Insert(&notificationBeforeV346{UserID: 1, RepoID: 1, Status: 1, Source: 99, UpdatedBy: 2})
	require.NoError(t, err)

	require.NoError(t, AddNotificationSubjectIdentity(x))

	var notifications []*NotificationV346
	require.NoError(t, x.Table("notification").Find(&notifications))
	require.Len(t, notifications, 1)
	assert.Equal(t, uint8(99), notifications[0].Source)
}
