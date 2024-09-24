// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activities

import (
	"context"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
)

type UserFeed struct {
	ID          int64              `xorm:"pk autoincr"`
	UserID      int64              `xorm:"UNIQUE(s)"` // Receiver user id.
	ActivityID  int64              `xorm:"UNIQUE(s)"` // refer to action table
	CreatedUnix timeutil.TimeStamp `xorm:"created"`
}

// DeleteOldUserFeeds deletes all old actions from database.
func DeleteOldUserFeeds(ctx context.Context, olderThan time.Duration) (err error) {
	if olderThan <= 0 {
		return nil
	}

	_, err = db.GetEngine(ctx).Where("created_unix < ?", time.Now().Add(-olderThan).Unix()).Delete(&UserFeed{})
	return err
}
