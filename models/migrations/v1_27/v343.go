// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"gitea.dev/models/db"

	"xorm.io/xorm"
)

// AddMaxParallelAndRunJobIndex adds the max_parallel column to action_run_job and a
// composite index on (run_id, job_id) to speed up max-parallel slot queries.
func AddMaxParallelAndRunJobIndex(x db.EngineMigration) error {
	type ActionRunJob struct {
		MaxParallel int    `xorm:"NOT NULL DEFAULT 0"`
		RunID       int64  `xorm:"index index(idx_run_id_job_id)"`
		JobID       string `xorm:"VARCHAR(255) index(idx_run_id_job_id)"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains:  true,
		IgnoreDropIndices: true,
	}, new(ActionRunJob))
	return err
}
