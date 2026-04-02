// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import "xorm.io/xorm"

func AddCloseReasonColumnsToIssue(x *xorm.Engine) error {
	type Issue struct {
		CloseReason      int64  `xorm:"INDEX DEFAULT 0"`
		CloseReasonParam string `xorm:"TEXT"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true}, new(Issue))
	return err
}
