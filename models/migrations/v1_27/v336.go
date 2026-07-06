// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"gitea.dev/models/db"
	"gitea.dev/modules/timeutil"
)

func AddActionRunJobSummaryTable(x db.EngineMigration) error {
	type ActionRunJobSummary struct {
		ID int64 `xorm:"pk autoincr"`

		RepoID       int64 `xorm:"UNIQUE(summary_key)"`
		RunID        int64 `xorm:"UNIQUE(summary_key)"`
		RunAttemptID int64 `xorm:"UNIQUE(summary_key) NOT NULL DEFAULT 0"`
		JobID        int64 `xorm:"UNIQUE(summary_key)"`
		StepIndex    int64 `xorm:"UNIQUE(summary_key)"`

		Content     string `xorm:"LONGTEXT"`
		ContentType string `xorm:"VARCHAR(255) NOT NULL DEFAULT 'text/markdown'"`
		ContentSize int64  `xorm:"NOT NULL DEFAULT 0"`

		Created timeutil.TimeStamp `xorm:"created"`
		Updated timeutil.TimeStamp `xorm:"updated"`
	}

	return x.Sync(new(ActionRunJobSummary))
}
