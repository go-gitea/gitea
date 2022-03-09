// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func addSessionTable(x *xorm.Engine) error {
	type Session struct {
		Key    string `xorm:"pk CHAR(16)"`
		Data   []byte `xorm:"BLOB"`
		Expiry timeutil.TimeStamp
	}
	return x.Sync2(new(Session))
}
