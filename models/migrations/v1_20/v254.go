// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_20 //nolint

import (
	"xorm.io/xorm"
)

func AddActionTaskOutputTable(x *xorm.Engine) error {
	type ActionTaskOutput struct {
		ID      int64
		TaskID  int64  `xorm:"INDEX UNIQUE(task_id_key)"`
		Key     string `xorm:"VARCHAR(255)"`
		KeyHash string `xorm:"CHAR(40) UNIQUE(task_id_key)"`
		Value   string `xorm:"TEXT"`
	}
	return x.Sync(new(ActionTaskOutput))
}
