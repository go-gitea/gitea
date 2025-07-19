// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_24

import (
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func AddFileStatusToAttachment(x *xorm.Engine) error {
	type Attachment struct {
		ID                   int64  `xorm:"pk autoincr"`
		UUID                 string `xorm:"uuid UNIQUE"`
		RepoID               int64  `xorm:"INDEX"`           // this should not be zero
		IssueID              int64  `xorm:"INDEX"`           // maybe zero when creating
		ReleaseID            int64  `xorm:"INDEX"`           // maybe zero when creating
		UploaderID           int64  `xorm:"INDEX DEFAULT 0"` // Notice: will be zero before this column added
		CommentID            int64  `xorm:"INDEX"`
		Name                 string
		DownloadCount        int64              `xorm:"DEFAULT 0"`
		Status               db.FileStatus      `xorm:"INDEX DEFAULT 0"`
		DeleteFailedCount    int                `xorm:"DEFAULT 0"` // Number of times the deletion failed, used to prevent infinite loop
		LastDeleteFailedTime timeutil.TimeStamp // Last time the deletion failed, used to prevent infinite loop
		Size                 int64              `xorm:"DEFAULT 0"`
		CreatedUnix          timeutil.TimeStamp `xorm:"created"`
		CustomDownloadURL    string             `xorm:"-"`
	}

	if err := x.Sync(new(Attachment)); err != nil {
		return err
	}

	if _, err := x.Exec("UPDATE `attachment` SET status = ? WHERE status IS NULL", db.FileStatusNormal); err != nil {
		return err
	}

	return nil
}
