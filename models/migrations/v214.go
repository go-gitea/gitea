// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func addAutoMergeTable(x *xorm.Engine) error {
	type MergeStyle string
	type PullAutoMerge struct {
		ID          int64      `xorm:"pk autoincr"`
		PullID      int64      `xorm:"UNIQUE"`
		DoerID      int64      `xorm:"NOT NULL"`
		MergeStyle  MergeStyle `xorm:"varchar(30)"`
		Message     string     `xorm:"LONGTEXT"`
		CreatedUnix int64      `xorm:"created"`
	}

	return x.Sync2(&PullAutoMerge{})
}
