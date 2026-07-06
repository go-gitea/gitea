// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_9

import (
	"gitea.dev/models/db"
	"gitea.dev/modules/timeutil"
)

func AddUploaderIDForAttachment(x db.EngineMigration) error {
	type Attachment struct {
		ID            int64  `xorm:"pk autoincr"`
		UUID          string `xorm:"uuid UNIQUE"`
		IssueID       int64  `xorm:"INDEX"`
		ReleaseID     int64  `xorm:"INDEX"`
		UploaderID    int64  `xorm:"INDEX DEFAULT 0"`
		CommentID     int64
		Name          string
		DownloadCount int64              `xorm:"DEFAULT 0"`
		Size          int64              `xorm:"DEFAULT 0"`
		CreatedUnix   timeutil.TimeStamp `xorm:"created"`
	}

	return x.Sync(new(Attachment))
}
