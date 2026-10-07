// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/timeutil"
)

// AddPushMirrorConfigAndHistory adds the config column and the history table of push mirrors.
func AddPushMirrorConfigAndHistory(_ context.Context, x base.EngineMigration) error {
	type PushMirror struct {
		Config string `xorm:"TEXT JSON"`
	}
	type PushMirrorHistory struct {
		ID           int64 `xorm:"pk autoincr"`
		RepoID       int64 `xorm:"INDEX"`
		PushMirrorID int64 `xorm:"INDEX"`
		Status       string
		DurationMs   int64
		Result       string             `xorm:"TEXT JSON"`
		CreatedUnix  timeutil.TimeStamp `xorm:"created"`
	}
	return x.Sync(new(PushMirror), new(PushMirrorHistory))
}
