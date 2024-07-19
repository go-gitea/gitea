// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22 //nolint

import (
	"xorm.io/xorm"
)

type HookTask struct {
	PayloadVersion int `xorm:"DEFAULT 1"`
}

func AddPayloadVersionToHookTaskTable(x *xorm.Engine) error {
	// create missing column
	if _, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreIndices:    true,
		IgnoreConstrains: true,
	}, new(HookTask)); err != nil {
		return err
	}
	_, err := x.Exec("UPDATE hook_task SET payload_version = 1 WHERE payload_version IS NULL")
	return err
}
