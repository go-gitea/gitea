// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func addRevisionTable(x *xorm.Engine) error {
	type Revision struct {
		ID              int64              `xorm:"pk autoincr"`
		Commit          string             `xorm:"NOT NULL"`
		NumberOfCommits int64              `xorm:"NOT NULL"`
		UserID          int64              `xorm:"NOT NULL"`
		PRID            int64              `xorm:"pr_id NOT NULL"`
		Index           int64              `xorm:"NOT NULL"`
		Created         timeutil.TimeStamp `xorm:"NOT NULL"`
	}
	return x.Sync2(new(Revision))
}
