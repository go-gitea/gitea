// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_17 //nolint

import (
	"xorm.io/xorm"
)

func AddAutoMergeTable(x *xorm.Engine) error {
	type MergeStyle string
	type PullAutoMerge struct {
		ID          int64      `xorm:"pk autoincr"`
		PullID      int64      `xorm:"UNIQUE"`
		DoerID      int64      `xorm:"NOT NULL"`
		MergeStyle  MergeStyle `xorm:"varchar(30)"`
		Message     string     `xorm:"LONGTEXT"`
		CreatedUnix int64      `xorm:"created"`
	}

	return x.Sync(&PullAutoMerge{})
}
