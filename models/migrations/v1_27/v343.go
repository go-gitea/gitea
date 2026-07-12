// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"gitea.dev/models/db"
	"gitea.dev/modules/timeutil"
)

func AddReleaseReactionTable(x db.EngineMigration) error {
	type ReleaseReaction struct {
		ID               int64              `xorm:"pk autoincr"`
		Type             string             `xorm:"INDEX UNIQUE(s) NOT NULL"`
		ReleaseID        int64              `xorm:"INDEX UNIQUE(s) NOT NULL"`
		UserID           int64              `xorm:"INDEX UNIQUE(s) NOT NULL"`
		OriginalAuthorID int64              `xorm:"INDEX UNIQUE(s) NOT NULL DEFAULT(0)"`
		OriginalAuthor   string             `xorm:"INDEX UNIQUE(s)"`
		CreatedUnix      timeutil.TimeStamp `xorm:"INDEX created"`
	}
	return x.Sync(new(ReleaseReaction))
}
