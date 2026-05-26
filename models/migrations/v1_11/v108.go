// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_11

import "gitea.dev/models/db"

func AddCommentIDOnNotification(x db.EngineMigration) error {
	type Notification struct {
		ID        int64 `xorm:"pk autoincr"`
		CommentID int64
	}

	return x.Sync(new(Notification))
}
