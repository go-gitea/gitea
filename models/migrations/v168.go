// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func restrictUserAndReactionColumnSize(x *xorm.Engine) error {
	type Reaction struct {
		ID             int64  `xorm:"pk autoincr"`
		Type           string `xorm:"VARCHAR(32) INDEX UNIQUE(s) NOT NULL"`
		OriginalAuthor string `xorm:"VARCHAR(64) INDEX UNIQUE(s)"`
	}
	if err := x.Sync2(new(Reaction)); err != nil {
		return err
	}

	type User struct {
		ID        int64  `xorm:"pk autoincr"`
		LowerName string `xorm:"VARCHAR(64) UNIQUE NOT NULL"`
		Name      string `xorm:"VARCHAR(64) UNIQUE NOT NULL"`
	}
	return x.Sync2(new(User))
}
