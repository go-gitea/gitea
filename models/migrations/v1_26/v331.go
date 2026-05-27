// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import "gitea.dev/models/db"

// AddJobMaxParallel adds max_parallel to action_run_job with a composite index on (run_id, job_id).
func AddJobMaxParallel(x db.EngineMigration) error {
	type ActionRunJob struct {
		RunID       int64  `xorm:"index index(idx_run_id_job_id)"`
		JobID       string `xorm:"VARCHAR(255) index(idx_run_id_job_id)"`
		MaxParallel int    `xorm:"NOT NULL DEFAULT 0"`
	}

	return x.Sync(new(ActionRunJob))
}
