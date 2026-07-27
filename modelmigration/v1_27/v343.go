// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"fmt"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/timeutil"

	"xorm.io/xorm/schemas"
)

type NotificationSourceV343 uint8

const (
	notificationSourceIssueV343 NotificationSourceV343 = iota + 1
	notificationSourcePullRequestV343
	notificationSourceCommitV343
	notificationSourceRepositoryV343
	notificationSourceReleaseV343
)

type notificationStatusV343 uint8

const (
	notificationStatusUnreadV343 notificationStatusV343 = iota + 1
	_                                                   // read (unused in merge logic)
	notificationStatusPinnedV343
)

type NotificationV343 struct { //revive:disable-line:exported
	ID     int64 `xorm:"pk autoincr"`
	UserID int64 `xorm:"NOT NULL"`
	RepoID int64 `xorm:"NOT NULL"`

	Status notificationStatusV343 `xorm:"SMALLINT NOT NULL"`
	Source NotificationSourceV343 `xorm:"SMALLINT NOT NULL"`

	IssueID   int64 `xorm:"NOT NULL"`
	CommitID  string
	CommentID int64
	ReleaseID int64
	UniqueKey string `xorm:"VARCHAR(255) NOT NULL"`

	UpdatedBy int64 `xorm:"NOT NULL"`

	CreatedUnix timeutil.TimeStamp `xorm:"created NOT NULL"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated NOT NULL"`
}

func (n *NotificationV343) TableName() string {
	return "notification"
}

// TableIndices implements xorm's TableIndices interface
func (n *NotificationV343) TableIndices() []*schemas.Index {
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

type NotificationV343Backfill struct { //revive:disable-line:exported
	ID     int64 `xorm:"pk autoincr"`
	UserID int64 `xorm:"NOT NULL"`
	RepoID int64 `xorm:"NOT NULL"`

	Status notificationStatusV343 `xorm:"SMALLINT NOT NULL"`
	Source NotificationSourceV343 `xorm:"SMALLINT NOT NULL"`

	IssueID   int64 `xorm:"NOT NULL"`
	CommitID  string
	CommentID int64
	ReleaseID int64
	UniqueKey *string `xorm:"VARCHAR(255) DEFAULT NULL"`

	UpdatedBy int64 `xorm:"NOT NULL"`

	CreatedUnix timeutil.TimeStamp `xorm:"created NOT NULL"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated NOT NULL"`
}

func (n *NotificationV343Backfill) TableName() string {
	return "notification"
}

func (n *NotificationV343Backfill) TableIndices() []*schemas.Index {
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

type notificationV343Duplicate struct {
	UserID    int64
	UniqueKey string
	Cnt       int64
}

// uniqueKeyV343 returns the unique_key for a notification row, falling back to a
// per-id placeholder so that rows with an unknown/unset source still receive a
// non-empty value before the column is switched to NOT NULL.
func uniqueKeyV343(id int64, source NotificationSourceV343, repoID, issueID, releaseID int64, commitID string) string {
	switch source {
	case notificationSourceIssueV343:
		return fmt.Sprintf("issue-%d", issueID)
	case notificationSourcePullRequestV343:
		return fmt.Sprintf("pull-%d", issueID)
	case notificationSourceCommitV343:
		return fmt.Sprintf("commit-%d-%s", repoID, commitID)
	case notificationSourceRepositoryV343:
		return fmt.Sprintf("repo-%d", repoID)
	case notificationSourceReleaseV343:
		return fmt.Sprintf("release-%d", releaseID)
	default:
		return fmt.Sprintf("legacy-%d", id)
	}
}

func backfillNotificationUniqueKeyV343(x base.EngineMigration) error {
	const batchSize = 1000
	lastID := int64(0)

	for {
		notifications := make([]*NotificationV343Backfill, 0, batchSize)
		if err := x.Where("id > ?", lastID).Asc("id").Limit(batchSize).Find(&notifications); err != nil {
			return err
		}
		if len(notifications) == 0 {
			return nil
		}

		for _, notification := range notifications {
			lastID = notification.ID

			uniqueKey := uniqueKeyV343(
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

// mergeNotificationStatusV343 collapses N duplicate-row statuses into one:
// Pinned wins over Unread which wins over Read.
func mergeNotificationStatusV343(notifications []*NotificationV343Backfill) notificationStatusV343 {
	mergedStatus := notifications[0].Status
	if mergedStatus == notificationStatusPinnedV343 {
		return notificationStatusPinnedV343
	}
	for _, notification := range notifications[1:] {
		if notification.Status == notificationStatusPinnedV343 {
			return notificationStatusPinnedV343
		}
		if notification.Status == notificationStatusUnreadV343 {
			mergedStatus = notificationStatusUnreadV343
		}
	}
	return mergedStatus
}

func dedupeNotificationRowsV343(x base.EngineMigration) error {
	var duplicatedNotifications []notificationV343Duplicate
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
		notifications := make([]*NotificationV343Backfill, 0, duplicatedNotification.Cnt)
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
		keeper.Status = mergeNotificationStatusV343(notifications)
		if _, err := x.ID(keeper.ID).Cols("status").NoAutoTime().Update(keeper); err != nil {
			return err
		}

		idsToDelete := make([]int64, 0, len(notifications)-1)
		for _, notification := range notifications[1:] {
			idsToDelete = append(idsToDelete, notification.ID)
		}
		if _, err := x.In("id", idsToDelete).Table("notification").Delete(&NotificationV343Backfill{}); err != nil {
			return err
		}
	}

	return nil
}

func AddReleaseNotification(x base.EngineMigration) error {
	if err := x.Sync(new(NotificationV343Backfill)); err != nil {
		return err
	}
	if err := backfillNotificationUniqueKeyV343(x); err != nil {
		return err
	}
	if err := dedupeNotificationRowsV343(x); err != nil {
		return err
	}
	return x.Sync(new(NotificationV343))
}
