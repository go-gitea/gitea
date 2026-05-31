// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_17

import (
	"time"

	"gitea.dev/models/db"
	"gitea.dev/models/repo"
	"gitea.dev/modules/timeutil"
)

func AddSyncOnCommitColForPushMirror(x db.EngineMigration) error {
	type PushMirror struct {
		ID         int64            `xorm:"pk autoincr"`
		RepoID     int64            `xorm:"INDEX"`
		Repo       *repo.Repository `xorm:"-"`
		RemoteName string

		SyncOnCommit   bool `xorm:"NOT NULL DEFAULT true"`
		Interval       time.Duration
		CreatedUnix    timeutil.TimeStamp `xorm:"created"`
		LastUpdateUnix timeutil.TimeStamp `xorm:"INDEX last_update"`
		LastError      string             `xorm:"text"`
	}

	return x.Sync(new(PushMirror))
}
