// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"gitea.dev/models/db"
	"gitea.dev/modules/timeutil"
)

type SavedReply struct {
	ID          int64              `xorm:"pk autoincr"`
	UserID      int64              `xorm:"INDEX NOT NULL"`
	Title       string             `xorm:"VARCHAR(255) NOT NULL"`
	Content     string             `xorm:"TEXT NOT NULL"`
	CreatedUnix timeutil.TimeStamp `xorm:"created"`
}

func (*SavedReply) TableName() string {
	return "user_saved_replies"
}

func AddSavedReplyTable(x db.EngineMigration) error {
	return x.Sync(new(SavedReply))
}
