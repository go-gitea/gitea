// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_28

import (
	"gitea.dev/models/db"

	"xorm.io/xorm"
)

// AddMatrixEvaluationColumnsToActionRunJob adds RawStrategy and IsMatrixEvaluated columns
// to support deferred matrix expansion for jobs whose matrix depends on other jobs' outputs.
func AddMatrixEvaluationColumnsToActionRunJob(x db.EngineMigration) error {
	type ActionRunJob struct {
		RawStrategy       string `xorm:"TEXT"`
		IsMatrixEvaluated bool
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	}, new(ActionRunJob))
	return err
}
