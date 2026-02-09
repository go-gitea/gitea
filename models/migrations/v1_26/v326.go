// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"xorm.io/xorm"
)

func AddMatrixEvaluationColumnsToActionRunJob(x *xorm.Engine) error {
	return x.Sync(new(ActionRunJobWithMatrixSupport))
}

// ActionRunJobWithMatrixSupport is a temporary struct for migration purposes
// It only defines the new columns we need to add
type ActionRunJobWithMatrixSupport struct {
	RawStrategy       string `xorm:"TEXT"` // raw strategy from job YAML's "strategy" section
	IsMatrixEvaluated bool   // whether the matrix has been evaluated with job outputs
}

// TableName returns the table name for xorm to sync
func (ActionRunJobWithMatrixSupport) TableName() string {
	return "action_run_job"
}
