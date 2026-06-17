// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"gitea.dev/models/db"

	"xorm.io/xorm"
)

// AddJobMaxParallel adds the MaxParallel column to ActionRunJob to support
// limiting how many matrix jobs from the same job definition run concurrently.
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
