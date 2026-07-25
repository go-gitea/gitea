// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_28

import (
	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

// AddMatrixDeferredColumnToActionRunJob adds the IsMatrixDeferred column, marking jobs whose
// matrix depends on other jobs' outputs and is therefore expanded only once those jobs finish.
func AddMatrixDeferredColumnToActionRunJob(x base.EngineMigration) error {
	type ActionRunJob struct {
		IsMatrixDeferred bool `xorm:"NOT NULL DEFAULT FALSE"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	}, new(ActionRunJob))
	return err
}
