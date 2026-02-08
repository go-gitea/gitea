// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"xorm.io/xorm"
)

func AddParentRunAndJobToActionRun(x *xorm.Engine) error {
	type ActionRun struct {
		ParentJobID int64 `xorm:"index"`
	}
	type ActionRunJob struct {
		ChildRunID int64 `xorm:"index"`
	}

	if _, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreIndices:    true,
		IgnoreConstrains: true,
	}, &ActionRun{}); err != nil {
		return err
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreIndices:    true,
		IgnoreConstrains: true,
	}, &ActionRunJob{})
	return err
}
