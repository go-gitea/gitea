// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"gitea.dev/models/db"

	"xorm.io/xorm"
)

// AddContinueOnErrorToActionRunJob adds the ContinueOnError column to ActionRunJob,
// storing the job-level continue-on-error value from the workflow YAML.
func AddContinueOnErrorToActionRunJob(x db.EngineMigration) error {
	type ActionRunJob struct {
		ContinueOnError bool `xorm:"NOT NULL DEFAULT FALSE"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	}, new(ActionRunJob))
	return err
}
