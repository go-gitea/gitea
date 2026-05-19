// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"xorm.io/xorm"
)

// AddMatrixEvaluationColumnsToActionRunJob adds RawStrategy and IsMatrixEvaluated columns
// to support deferred matrix expansion for jobs whose matrix depends on other jobs' outputs.
func AddMatrixEvaluationColumnsToActionRunJob(x *xorm.Engine) error {
	// ActionRunJob maps to the "action_run_job" table by xorm naming convention.
	type ActionRunJob struct {
		RawStrategy       string `xorm:"TEXT"`
		IsMatrixEvaluated bool
	}
	if _, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
	}, new(ActionRunJob)); err != nil {
		return err
	}
	return nil
}
