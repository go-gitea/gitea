// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"context"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/timeutil"

	"xorm.io/xorm"
)

// AddQueueRankToActionRunJob adds the QueueRank column to ActionRunJob, used to manually
// reorder waiting jobs in the build queue. All existing jobs default to 0 (natural FIFO order).
//
// It also adds the "pickup" composite index (task_id, status, queue_rank, updated) matching the
// runner-poll query's WHERE task_id=0 AND status=waiting ORDER BY queue_rank, updated, id: queue_rank
// alone is a poor sort key (0 for nearly every row), so task_id/status must lead it to stay index-ordered.
func AddQueueRankToActionRunJob(_ context.Context, x base.EngineMigration) error {
	type ActionRunJob struct {
		TaskID    int64              `xorm:"index(pickup)"`
		Status    int                `xorm:"index(pickup)"`
		QueueRank int64              `xorm:"index index(pickup) NOT NULL DEFAULT 0"`
		Updated   timeutil.TimeStamp `xorm:"index(pickup)"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	}, new(ActionRunJob))
	return err
}
