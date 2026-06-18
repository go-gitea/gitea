// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import "gitea.dev/models/db"

// AddEnvironmentNameToActionRunJob adds environment_name column to action_run_job
// to track the deployment environment a job targets.
func AddEnvironmentNameToActionRunJob(x db.EngineMigration) error {
	type ActionRunJob struct {
		EnvironmentName string `xorm:"VARCHAR(255) NOT NULL DEFAULT ''"`
	}
	return x.Sync(new(ActionRunJob))
}
