// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_9 //nolint:revive // underscore in migration packages isn't a large issue

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func AddUploaderIDForAttachment(x *xorm.Engine) error {
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
