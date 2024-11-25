// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23 //nolint

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

type improveNotificationTableIndicesAction struct {
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

// TableName sets the name of this table
func (*improveNotificationTableIndicesAction) TableName() string {
	return "notification"
}

// TableIndices implements xorm's TableIndices interface
func (*improveNotificationTableIndicesAction) TableIndices() []*schemas.Index {
	indices := make([]*schemas.Index, 0, 8)
	usuuIndex := schemas.NewIndex("u_s_uu", schemas.IndexType)
	usuuIndex.AddColumn("user_id", "status", "updated_unix")
	indices = append(indices, usuuIndex)

	// Add the individual indices that were previously defined in struct tags
	userIDIndex := schemas.NewIndex("idx_notification_user_id", schemas.IndexType)
	userIDIndex.AddColumn("user_id")
	indices = append(indices, userIDIndex)

	repoIDIndex := schemas.NewIndex("idx_notification_repo_id", schemas.IndexType)
	repoIDIndex.AddColumn("repo_id")
	indices = append(indices, repoIDIndex)

	statusIndex := schemas.NewIndex("idx_notification_status", schemas.IndexType)
	statusIndex.AddColumn("status")
	indices = append(indices, statusIndex)

	sourceIndex := schemas.NewIndex("idx_notification_source", schemas.IndexType)
	sourceIndex.AddColumn("source")
	indices = append(indices, sourceIndex)

	issueIDIndex := schemas.NewIndex("idx_notification_issue_id", schemas.IndexType)
	issueIDIndex.AddColumn("issue_id")
	indices = append(indices, issueIDIndex)

	commitIDIndex := schemas.NewIndex("idx_notification_commit_id", schemas.IndexType)
	commitIDIndex.AddColumn("commit_id")
	indices = append(indices, commitIDIndex)

	updatedByIndex := schemas.NewIndex("idx_notification_updated_by", schemas.IndexType)
	updatedByIndex.AddColumn("updated_by")
	indices = append(indices, updatedByIndex)

	return indices
}

func ImproveNotificationTableIndices(x *xorm.Engine) error {
	return x.Sync(&improveNotificationTableIndicesAction{})
}
