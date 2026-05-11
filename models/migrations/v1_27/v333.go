// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"fmt"

	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

type NotificationSourceV331 uint8

const (
	notificationSourceIssueV331 NotificationSourceV331 = iota + 1
	notificationSourcePullRequestV331
	notificationSourceCommitV331
	notificationSourceRepositoryV331
	notificationSourceReleaseV331
)

type notificationStatusV331 uint8

const (
	notificationStatusUnreadV331 notificationStatusV331 = iota + 1
	_                                                   // read (unused in merge logic)
	notificationStatusPinnedV331
)

type NotificationV331 struct { //revive:disable-line:exported
	ID     int64 `xorm:"pk autoincr"`
	UserID int64 `xorm:"NOT NULL"`
	RepoID int64 `xorm:"NOT NULL"`

	Status notificationStatusV331 `xorm:"SMALLINT NOT NULL"`
	Source NotificationSourceV331 `xorm:"SMALLINT NOT NULL"`

	IssueID   int64 `xorm:"NOT NULL"`
	CommitID  string
	CommentID int64
	ReleaseID int64
	UniqueKey *string `xorm:"VARCHAR(255) DEFAULT NULL"`

	UpdatedBy int64 `xorm:"NOT NULL"`

	CreatedUnix timeutil.TimeStamp `xorm:"created NOT NULL"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated NOT NULL"`
}

func (n *NotificationV331) TableName() string {
	return "notification"
}

// TableIndices implements xorm's TableIndices interface
func (n *NotificationV331) TableIndices() []*schemas.Index {
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

type NotificationV331Backfill struct { //revive:disable-line:exported
	ID     int64 `xorm:"pk autoincr"`
	UserID int64 `xorm:"NOT NULL"`
	RepoID int64 `xorm:"NOT NULL"`

	Status notificationStatusV331 `xorm:"SMALLINT NOT NULL"`
	Source NotificationSourceV331 `xorm:"SMALLINT NOT NULL"`

	IssueID   int64 `xorm:"NOT NULL"`
	CommitID  string
	CommentID int64
	ReleaseID int64
	UniqueKey *string `xorm:"VARCHAR(255) DEFAULT NULL"`

	UpdatedBy int64 `xorm:"NOT NULL"`

	CreatedUnix timeutil.TimeStamp `xorm:"created NOT NULL"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated NOT NULL"`
}

func (n *NotificationV331Backfill) TableName() string {
	return "notification"
}

func (n *NotificationV331Backfill) TableIndices() []*schemas.Index {
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

type notificationV331Duplicate struct {
	UserID    int64
	UniqueKey string
	Cnt       int64
}

func uniqueKeyV331(source NotificationSourceV331, repoID, issueID, releaseID int64, commitID string) *string {
	var key string
	switch source {
	case notificationSourceIssueV331:
		key = fmt.Sprintf("issue-%d", issueID)
	case notificationSourcePullRequestV331:
		key = fmt.Sprintf("pull-%d", issueID)
	case notificationSourceCommitV331:
		key = fmt.Sprintf("commit-%d-%s", repoID, commitID)
	case notificationSourceReleaseV331:
		key = fmt.Sprintf("release-%d", releaseID)
	default:
		return nil
	}

	return &key
}

func backfillNotificationUniqueKeyV331(x *xorm.Engine) error {
	const batchSize = 50
	lastID := int64(0)

	for {
		notifications := make([]*NotificationV331Backfill, 0, batchSize)
		if err := x.Where("id > ?", lastID).Asc("id").Limit(batchSize).Find(&notifications); err != nil {
			return err
		}
		if len(notifications) == 0 {
			return nil
		}

		for _, notification := range notifications {
			lastID = notification.ID

			uniqueKey := uniqueKeyV331(
				notification.Source,
				notification.RepoID,
				notification.IssueID,
				notification.ReleaseID,
				notification.CommitID,
			)
			if uniqueKey == nil {
				continue
			}

			if _, err := x.Exec(
				"UPDATE notification SET unique_key = ? WHERE id = ?",
				*uniqueKey,
				notification.ID,
			); err != nil {
				return err
			}
		}
	}
}

func mergeNotificationStatusV331(notifications []*NotificationV331Backfill) notificationStatusV331 {
	mergedStatus := notifications[0].Status
	for _, notification := range notifications[1:] {
		switch {
		case notification.Status == notificationStatusPinnedV331:
			return notificationStatusPinnedV331
		case notification.Status == notificationStatusUnreadV331 && mergedStatus != notificationStatusPinnedV331:
			mergedStatus = notificationStatusUnreadV331
		}
	}
	return mergedStatus
}

func dedupeNotificationRowsV331(x *xorm.Engine) error {
	var duplicatedNotifications []notificationV331Duplicate
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
		notifications := make([]*NotificationV331Backfill, 0, duplicatedNotification.Cnt)
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
		keeper.Status = mergeNotificationStatusV331(notifications)
		if _, err := x.ID(keeper.ID).Cols("status").NoAutoTime().Update(keeper); err != nil {
			return err
		}

		idsToDelete := make([]int64, 0, len(notifications)-1)
		for _, notification := range notifications[1:] {
			idsToDelete = append(idsToDelete, notification.ID)
		}
		if _, err := x.In("id", idsToDelete).Table("notification").Delete(&NotificationV331Backfill{}); err != nil {
			return err
		}
	}

	return nil
}

func AddReleaseNotification(x *xorm.Engine) error {
	if err := x.Sync(new(NotificationV331Backfill)); err != nil {
		return err
	}
	if err := backfillNotificationUniqueKeyV331(x); err != nil {
		return err
	}
	if err := dedupeNotificationRowsV331(x); err != nil {
		return err
	}
	return x.Sync(new(NotificationV331))
}
