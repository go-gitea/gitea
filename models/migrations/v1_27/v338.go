// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"gitea.dev/models/db"

	"xorm.io/xorm"
)

// AddJobMaxParallel adds the max_parallel column to action_run_job.
func AddJobMaxParallel(x db.EngineMigration) error {
	type ActionRunJob struct {
		MaxParallel int `xorm:"NOT NULL DEFAULT 0"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains:  true,
		IgnoreDropIndices: true,
	}, new(ActionRunJob))
	return err
}
