// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"context"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/timeutil"

	"xorm.io/xorm"
)

// AddPickupIndexToActionRunJob adds the "pickup" composite index (task_id, status, updated) matching
// the runner-poll query's WHERE task_id=0 AND status=waiting ORDER BY updated, id.
func AddPickupIndexToActionRunJob(_ context.Context, x base.EngineMigration) error {
	type ActionRunJob struct {
		TaskID  int64              `xorm:"index(pickup)"`
		Status  int                `xorm:"index(pickup)"`
		Updated timeutil.TimeStamp `xorm:"index(pickup)"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	}, new(ActionRunJob))
	return err
}
