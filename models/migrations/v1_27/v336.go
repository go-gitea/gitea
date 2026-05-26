// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"code.gitea.io/gitea/models/db"

	"xorm.io/xorm"
)

func AddCommitCommentIDToNotification(x db.EngineMigration) error {
	type Notification struct {
		CommitCommentID int64
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains:  true,
		IgnoreDropIndices: true,
	}, new(Notification))
	return err
}
