// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22

import (
	"gitea.dev/models/db"

	"xorm.io/xorm"
)

func AddCommentIDIndexofAttachment(x db.EngineMigration) error {
	type Attachment struct {
		CommentID int64 `xorm:"INDEX"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	}, &Attachment{})
	return err
}
