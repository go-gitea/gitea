// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_20 //nolint

import (
	"xorm.io/xorm"
)

func AddActionTaskOutputTable(x *xorm.Engine) error {
	type ActionTaskOutput struct {
		ID          int64
		TaskID      int64  `xorm:"INDEX UNIQUE(task_id_output_key)"`
		OutputKey   string `xorm:"VARCHAR(255) UNIQUE(task_id_output_key)"`
		OutputValue string `xorm:"MEDIUMTEXT"`
	}
	return x.Sync(new(ActionTaskOutput))
}
