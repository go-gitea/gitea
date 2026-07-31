// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_28

import (
	"context"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/timeutil"

	"xorm.io/xorm/schemas"
)

// Notification sources, mirrored from models/activities at the time of this migration.
const (
	notificationSourceIssueV345       = 1
	notificationSourcePullRequestV345 = 2
	notificationSourceCommitV345      = 3
	notificationSourceRepositoryV345  = 4
)

// Notification statuses, mirrored from models/activities at the time of this migration.
const (
	notificationStatusUnreadV345 = 1
	notificationStatusPinnedV345 = 3
)

// notificationV345Migrating carries both the old per-source subject columns and the new
// identity columns, so the backfill can read one and write the other.
type notificationV345Migrating struct {
	ID       int64 `xorm:"pk autoincr"`
	UserID   int64 `xorm:"NOT NULL"`
	RepoID   int64 `xorm:"NOT NULL"`
	Status   uint8 `xorm:"SMALLINT NOT NULL"`
	Source   uint8 `xorm:"SMALLINT NOT NULL"`
	IssueID  int64 `xorm:"NOT NULL"`
	CommitID string

	SubjectID  int64  `xorm:"NOT NULL DEFAULT 0"`
	SubjectRef string `xorm:"VARCHAR(64) NOT NULL DEFAULT ''"`
	Title      string `xorm:"VARCHAR(255) NOT NULL DEFAULT ''"`

	UpdatedUnix timeutil.TimeStamp `xorm:"updated NOT NULL"`
}

func (n *notificationV345Migrating) TableName() string {
	return "notification"
}

// NotificationV345 is the shape after this migration: a notification is identified by
// (user_id, repo_id, source, subject_id, subject_ref) and carries the title it renders with.
type NotificationV345 struct { //revive:disable-line:exported
	ID     int64 `xorm:"pk autoincr"`
	UserID int64 `xorm:"NOT NULL"`
	RepoID int64 `xorm:"NOT NULL"`

	Status uint8 `xorm:"SMALLINT NOT NULL"`
	Source uint8 `xorm:"SMALLINT NOT NULL"`

	SubjectID  int64  `xorm:"NOT NULL DEFAULT 0"`
	SubjectRef string `xorm:"VARCHAR(64) NOT NULL DEFAULT ''"`
	Title      string `xorm:"VARCHAR(255) NOT NULL DEFAULT ''"`

	CommentID int64
	UpdatedBy int64 `xorm:"NOT NULL"`

	CreatedUnix timeutil.TimeStamp `xorm:"created NOT NULL"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated NOT NULL"`
}

func (n *NotificationV345) TableName() string {
	return "notification"
}

// TableIndices implements xorm's TableIndices interface
func (n *NotificationV345) TableIndices() []*schemas.Index {
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

	uniqueSubject := schemas.NewIndex("unique_notification_subject", schemas.UniqueType)
	uniqueSubject.AddColumn("user_id", "repo_id", "source", "subject_id", "subject_ref")
	indices = append(indices, uniqueSubject)

	return indices
}

// AddNotificationSubjectIdentity replaces the per-source subject columns of the
// notification table with a (source, subject_id, subject_ref) identity plus a snapshotted
// title, so a notification can be rendered without loading its subject.
func AddNotificationSubjectIdentity(x base.EngineMigration) error {
	// 1. add the new columns alongside the old ones
	if err := x.Sync(new(notificationV345Migrating)); err != nil {
		return err
	}

	// 2. backfill in bulk — one statement per source rather than one per row
	if err := backfillNotificationSubjectV345(x); err != nil {
		return err
	}

	// 3. collapse rows that the new unique index would reject
	if err := dedupeNotificationsV345(x); err != nil {
		return err
	}

	// 4. create the unique index, then drop the columns it replaces
	if err := x.Sync(new(NotificationV345)); err != nil {
		return err
	}
	return dropNotificationLegacyColumnsV345(x)
}

func backfillNotificationSubjectV345(x base.EngineMigration) error {
	// issues and pull requests keep their issue id
	if _, err := x.Exec(
		"UPDATE notification SET subject_id = issue_id WHERE source IN (?, ?)",
		notificationSourceIssueV345, notificationSourcePullRequestV345,
	); err != nil {
		return err
	}

	// commits are identified by their sha, scoped to the repo by repo_id
	if _, err := x.Exec(
		"UPDATE notification SET subject_ref = commit_id WHERE source = ? AND commit_id IS NOT NULL",
		notificationSourceCommitV345,
	); err != nil {
		return err
	}

	// a repository notification needs no subject: repo_id already identifies it

	// Titles are deliberately left empty. The renderer falls back to loading the subject
	// when the title is empty, so existing notifications keep working and no per-row
	// backfill is needed here.
	return nil
}

type notificationDuplicateV345 struct {
	UserID     int64
	RepoID     int64
	Source     uint8
	SubjectID  int64
	SubjectRef string
	Cnt        int
}

// dedupeNotificationsV345 collapses duplicate rows that the old schema allowed but the new
// unique index forbids, keeping the most recently updated one. Same group-then-delete
// shape as AddUniqueIndexForUserBadge (v1_26/v329.go).
func dedupeNotificationsV345(x base.EngineMigration) error {
	var duplicates []notificationDuplicateV345
	if err := x.Select("user_id, repo_id, source, subject_id, subject_ref, count(*) as cnt").
		Table("notification").
		GroupBy("user_id, repo_id, source, subject_id, subject_ref").
		Having("count(*) > 1").
		Find(&duplicates); err != nil {
		return err
	}

	for _, duplicate := range duplicates {
		var rows []*notificationV345Migrating
		if err := x.Table("notification").
			Where("user_id = ?", duplicate.UserID).
			And("repo_id = ?", duplicate.RepoID).
			And("source = ?", duplicate.Source).
			And("subject_id = ?", duplicate.SubjectID).
			And("subject_ref = ?", duplicate.SubjectRef).
			Desc("updated_unix", "id").
			Find(&rows); err != nil {
			return err
		}
		if len(rows) < 2 {
			continue
		}

		keeper := rows[0]
		if status := mergeNotificationStatusV345(rows); status != keeper.Status {
			if _, err := x.Exec("UPDATE notification SET status = ? WHERE id = ?", status, keeper.ID); err != nil {
				return err
			}
		}

		ids := make([]int64, 0, len(rows)-1)
		for _, row := range rows[1:] {
			ids = append(ids, row.ID)
		}
		if _, err := x.Table("notification").In("id", ids).Delete(); err != nil {
			return err
		}
	}

	return nil
}

// mergeNotificationStatusV345 picks the status the surviving row keeps: pinned wins over
// unread, which wins over read, so collapsing rows never hides something unseen.
func mergeNotificationStatusV345(rows []*notificationV345Migrating) uint8 {
	merged := rows[0].Status
	for _, row := range rows {
		if row.Status == notificationStatusPinnedV345 {
			return notificationStatusPinnedV345
		}
		if row.Status == notificationStatusUnreadV345 {
			merged = notificationStatusUnreadV345
		}
	}
	return merged
}

func dropNotificationLegacyColumnsV345(x base.EngineMigration) error {
	columns := make([]string, 0, 2)
	for _, col := range []string{"issue_id", "commit_id"} {
		exist, err := x.Dialect().IsColumnExist(x.DB(), context.Background(), "notification", col)
		if err != nil {
			return err
		}
		if exist {
			columns = append(columns, col)
		}
	}
	if len(columns) == 0 {
		return nil
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := base.DropTableColumns(sess, "notification", columns...); err != nil {
		return err
	}
	// DropTableColumns rebuilds the table on SQLite, which drops all existing indexes.
	// Re-sync to restore the indexes defined on NotificationV345.
	return x.Sync(new(NotificationV345))
}
