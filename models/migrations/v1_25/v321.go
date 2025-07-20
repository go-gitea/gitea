// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_25

import (
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

type Attachment321 struct {
	ID                   int64  `xorm:"pk autoincr"`
	UUID                 string `xorm:"uuid UNIQUE"`
	RepoID               int64  `xorm:"INDEX"`           // this should not be zero
	IssueID              int64  `xorm:"INDEX"`           // maybe zero when creating
	ReleaseID            int64  `xorm:"INDEX"`           // maybe zero when creating
	UploaderID           int64  `xorm:"INDEX DEFAULT 0"` // Notice: will be zero before this column added
	CommentID            int64  `xorm:"INDEX"`
	Name                 string
	DownloadCount        int64              `xorm:"DEFAULT 0"`
	Status               db.FileStatus      `xorm:"INDEX DEFAULT 1 NOT NULL"` // 1 = normal, 2 = to be deleted
	DeleteFailedCount    int                `xorm:"DEFAULT 0"`                // Number of times the deletion failed, used to prevent infinite loop
	LastDeleteFailedTime timeutil.TimeStamp // Last time the deletion failed, used to prevent infinite loop
	Size                 int64              `xorm:"DEFAULT 0"`
	CreatedUnix          timeutil.TimeStamp `xorm:"created"`
}

func (a *Attachment321) TableName() string {
	return "attachment"
}

// TableIndices implements xorm's TableIndices interface
func (a *Attachment321) TableIndices() []*schemas.Index {
	uuidIndex := schemas.NewIndex("attachment_uuid", schemas.UniqueType)
	uuidIndex.AddColumn("uuid")

	repoIndex := schemas.NewIndex("attachment_repo_id", schemas.IndexType)
	repoIndex.AddColumn("repo_id")

	issueIndex := schemas.NewIndex("attachment_issue_id", schemas.IndexType)
	issueIndex.AddColumn("issue_id")

	releaseIndex := schemas.NewIndex("attachment_release_id", schemas.IndexType)
	releaseIndex.AddColumn("release_id")

	uploaderIndex := schemas.NewIndex("attachment_uploader_id", schemas.IndexType)
	uploaderIndex.AddColumn("uploader_id")

	commentIndex := schemas.NewIndex("attachment_comment_id", schemas.IndexType)
	commentIndex.AddColumn("comment_id")

	statusIndex := schemas.NewIndex("attachment_status", schemas.IndexType)
	statusIndex.AddColumn("status")

	statusIDIndex := schemas.NewIndex("attachment_status_id", schemas.IndexType)
	statusIDIndex.AddColumn("status_id", "id") // For status = ? AND id > ? query

	return []*schemas.Index{
		uuidIndex,
		repoIndex,
		issueIndex,
		releaseIndex,
		uploaderIndex,
		commentIndex,
		statusIndex,
		statusIDIndex,
	}
}

func AddFileStatusToAttachment(x *xorm.Engine) error {
	return x.Sync(new(Attachment321))
}
