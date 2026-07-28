// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_28

import (
	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

// AddDeferredMatrixColumnsToActionRunJob adds the columns backing deferred (dynamic) matrix expansion
func AddDeferredMatrixColumnsToActionRunJob(x base.EngineMigration) error {
	type ActionRunJob struct {
		// IsMatrixDeferred marks jobs whose matrix depends on other jobs' outputs and is therefore expanded only once those jobs finish;
		IsMatrixDeferred bool `xorm:"NOT NULL DEFAULT FALSE"`
		// DeferredMatrixPayload preserves the raw, unevaluated payload across expansion so a rerun can re-derive the matrix
		DeferredMatrixPayload []byte `xorm:"LONGBLOB"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	}, new(ActionRunJob))
	return err
}
