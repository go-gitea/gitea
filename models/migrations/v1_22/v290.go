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
	return x.Sync(new(HookTask))
}
