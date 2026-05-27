// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import "gitea.dev/models/db"

// AddJobMaxParallel adds the max_parallel column to action_run_job.
func AddJobMaxParallel(x db.EngineMigration) error {
	type ActionRunJob struct {
		MaxParallel int `xorm:"NOT NULL DEFAULT 0"`
	}

	return x.Sync(new(ActionRunJob))
}
