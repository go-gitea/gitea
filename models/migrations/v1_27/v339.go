// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"gitea.dev/models/db"

	"xorm.io/xorm"
)

// AddRunJobRunIDJobIDIndex adds a composite index on (run_id, job_id) to action_run_job
// to speed up max-parallel slot queries that filter by run_id and group by job_id.
func AddRunJobRunIDJobIDIndex(x db.EngineMigration) error {
	type ActionRunJob struct {
		RunID int64  `xorm:"index index(idx_run_id_job_id)"`
		JobID string `xorm:"VARCHAR(255) index(idx_run_id_job_id)"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains:  true,
		IgnoreDropIndices: true,
	}, new(ActionRunJob))
	return err
}