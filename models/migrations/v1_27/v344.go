// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"gitea.dev/models/db"

	"xorm.io/xorm"
)

// AddCommitCommentIDToNotification adds a dedicated column for linking a
// notification to a standalone commit comment. The existing comment_id column
// stays reserved for the issue/PR Comment table.
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
