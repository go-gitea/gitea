// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

// AddQueueRankToActionRunJob adds the QueueRank column to ActionRunJob, used to manually
// reorder waiting jobs in the build queue. All existing jobs default to 0 (natural FIFO order).
func AddQueueRankToActionRunJob(x base.EngineMigration) error {
	type ActionRunJob struct {
		QueueRank int64 `xorm:"index NOT NULL DEFAULT 0"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	}, new(ActionRunJob))
	return err
}
