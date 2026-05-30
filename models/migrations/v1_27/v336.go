// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"fmt"

	"gitea.dev/models/db"
	"gitea.dev/modules/timeutil"

	"xorm.io/xorm/schemas"
)

type NotificationSourceV336 uint8

const (
	notificationSourceIssueV336 NotificationSourceV336 = iota + 1
	notificationSourcePullRequestV336
	notificationSourceCommitV336
	notificationSourceRepositoryV336
	notificationSourceReleaseV336
)

type notificationStatusV336 uint8

const (
	notificationStatusUnreadV336 notificationStatusV336 = iota + 1
	_                                                   // read (unused in merge logic)
	notificationStatusPinnedV336
)

type NotificationV336 struct { //revive:disable-line:exported
	ID     int64 `xorm:"pk autoincr"`
	UserID int64 `xorm:"NOT NULL"`
	RepoID int64 `xorm:"NOT NULL"`

	Status notificationStatusV336 `xorm:"SMALLINT NOT NULL"`
	Source NotificationSourceV336 `xorm:"SMALLINT NOT NULL"`

	IssueID   int64 `xorm:"NOT NULL"`
	CommitID  string
	CommentID int64
	ReleaseID int64
	UniqueKey string `xorm:"VARCHAR(255) NOT NULL"`

	UpdatedBy int64 `xorm:"NOT NULL"`

	CreatedUnix timeutil.TimeStamp `xorm:"created NOT NULL"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated NOT NULL"`
}

func (n *NotificationV336) TableName() string {
	return "notification"
}

// TableIndices implements xorm's TableIndices interface
func (n *NotificationV336) TableIndices() []*schemas.Index {
	indices := make([]*schemas.Index, 0, 6)
	usuuIndex := schemas.NewIndex("u_s_uu", schemas.IndexType)
	usuuIndex.AddColumn("user_id", "status", "updated_unix")
	indices = append(indices, usuuIndex)

	userIDIndex := schemas.NewIndex("idx_notification_user_id", schemas.IndexType)
	userIDIndex.AddColumn("user_id")
	indices = append(indices, userIDIndex)

	repoIDIndex := schemas.NewIndex("idx_notification_repo_id", schemas.IndexType)
	repoIDIndex.AddColumn("repo_id")
	indices = append(indices, repoIDIndex)

	statusIndex := schemas.NewIndex("idx_notification_status", schemas.IndexType)
	statusIndex.AddColumn("status")
	indices = append(indices, statusIndex)

	updatedByIndex := schemas.NewIndex("idx_notification_updated_by", schemas.IndexType)
	updatedByIndex.AddColumn("updated_by")
	indices = append(indices, updatedByIndex)

	uniqueNotificationKey := schemas.NewIndex("unique_notification_key", schemas.UniqueType)
	uniqueNotificationKey.AddColumn("user_id", "unique_key")
	indices = append(indices, uniqueNotificationKey)

	return indices
}

type NotificationV336Backfill struct { //revive:disable-line:exported
	ID     int64 `xorm:"pk autoincr"`
	UserID int64 `xorm:"NOT NULL"`
	RepoID int64 `xorm:"NOT NULL"`

	Status notificationStatusV336 `xorm:"SMALLINT NOT NULL"`
	Source NotificationSourceV336 `xorm:"SMALLINT NOT NULL"`

	IssueID   int64 `xorm:"NOT NULL"`
	CommitID  string
	CommentID int64
	ReleaseID int64
	UniqueKey *string `xorm:"VARCHAR(255) DEFAULT NULL"`

	UpdatedBy int64 `xorm:"NOT NULL"`

	CreatedUnix timeutil.TimeStamp `xorm:"created NOT NULL"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated NOT NULL"`
}

func (n *NotificationV336Backfill) TableName() string {
	return "notification"
}

func (n *NotificationV336Backfill) TableIndices() []*schemas.Index {
	indices := make([]*schemas.Index, 0, 3)
	usuuIndex := schemas.NewIndex("u_s_uu", schemas.IndexType)
	usuuIndex.AddColumn("user_id", "status", "updated_unix")
	indices = append(indices, usuuIndex)

	repoIDIndex := schemas.NewIndex("idx_notification_repo_id", schemas.IndexType)
	repoIDIndex.AddColumn("repo_id")
	indices = append(indices, repoIDIndex)

	updatedByIndex := schemas.NewIndex("idx_notification_updated_by", schemas.IndexType)
	updatedByIndex.AddColumn("updated_by")
	indices = append(indices, updatedByIndex)

	return indices
}

type notificationV336Duplicate struct {
	UserID    int64
	UniqueKey string
	Cnt       int64
}

// uniqueKeyV336 returns the unique_key for a notification row, falling back to a
// per-id placeholder so that rows with an unknown/unset source still receive a
// non-empty value before the column is switched to NOT NULL.
func uniqueKeyV336(id int64, source NotificationSourceV336, repoID, issueID, releaseID int64, commitID string) string {
	switch source {
	case notificationSourceIssueV336:
		return fmt.Sprintf("issue-%d", issueID)
	case notificationSourcePullRequestV336:
		return fmt.Sprintf("pull-%d", issueID)
	case notificationSourceCommitV336:
		return fmt.Sprintf("commit-%d-%s", repoID, commitID)
	case notificationSourceRepositoryV336:
		return fmt.Sprintf("repo-%d", repoID)
	case notificationSourceReleaseV336:
		return fmt.Sprintf("release-%d", releaseID)
	default:
		return fmt.Sprintf("legacy-%d", id)
	}
}

func backfillNotificationUniqueKeyV336(x db.EngineMigration) error {
	const batchSize = 1000
	lastID := int64(0)

	for {
		notifications := make([]*NotificationV336Backfill, 0, batchSize)
		if err := x.Where("id > ?", lastID).Asc("id").Limit(batchSize).Find(&notifications); err != nil {
			return err
		}
		if len(notifications) == 0 {
			return nil
		}

		for _, notification := range notifications {
			lastID = notification.ID

			uniqueKey := uniqueKeyV336(
				notification.ID,
				notification.Source,
				notification.RepoID,
				notification.IssueID,
				notification.ReleaseID,
				notification.CommitID,
			)

			if _, err := x.Exec(
				"UPDATE notification SET unique_key = ? WHERE id = ?",
				uniqueKey,
				notification.ID,
			); err != nil {
				return err
			}
		}
	}
}

// mergeNotificationStatusV336 collapses N duplicate-row statuses into one:
// Pinned wins over Unread which wins over Read.
func mergeNotificationStatusV336(notifications []*NotificationV336Backfill) notificationStatusV336 {
	mergedStatus := notifications[0].Status
	if mergedStatus == notificationStatusPinnedV336 {
		return notificationStatusPinnedV336
	}
	for _, notification := range notifications[1:] {
		if notification.Status == notificationStatusPinnedV336 {
			return notificationStatusPinnedV336
		}
		if notification.Status == notificationStatusUnreadV336 {
			mergedStatus = notificationStatusUnreadV336
		}
	}
	return mergedStatus
}

func dedupeNotificationRowsV336(x db.EngineMigration) error {
	var duplicatedNotifications []notificationV336Duplicate
	if err := x.SQL(`
		SELECT user_id, unique_key, COUNT(1) AS cnt
		FROM notification
		WHERE unique_key IS NOT NULL
		GROUP BY user_id, unique_key
		HAVING COUNT(1) > 1
	`).Find(&duplicatedNotifications); err != nil {
		return err
	}

	for _, duplicatedNotification := range duplicatedNotifications {
		notifications := make([]*NotificationV336Backfill, 0, duplicatedNotification.Cnt)
		if err := x.Where("user_id = ?", duplicatedNotification.UserID).
			And("unique_key = ?", duplicatedNotification.UniqueKey).
			Desc("updated_unix", "id").
			Find(&notifications); err != nil {
			return err
		}
		if len(notifications) < 2 {
			continue
		}

		keeper := notifications[0]
		keeper.Status = mergeNotificationStatusV336(notifications)
		if _, err := x.ID(keeper.ID).Cols("status").NoAutoTime().Update(keeper); err != nil {
			return err
		}

		idsToDelete := make([]int64, 0, len(notifications)-1)
		for _, notification := range notifications[1:] {
			idsToDelete = append(idsToDelete, notification.ID)
		}
		if _, err := x.In("id", idsToDelete).Table("notification").Delete(&NotificationV336Backfill{}); err != nil {
			return err
		}
	}

	return nil
}

func AddReleaseNotification(x db.EngineMigration) error {
	if err := x.Sync(new(NotificationV336Backfill)); err != nil {
		return err
	}
	if err := backfillNotificationUniqueKeyV336(x); err != nil {
		return err
	}
	if err := dedupeNotificationRowsV336(x); err != nil {
		return err
	}
	return x.Sync(new(NotificationV336))
}
