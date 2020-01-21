// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func addLockedResourceTable(x *xorm.Engine) error {

	type LockedResource struct {
		LockType	string      `xorm:"pk VARCHAR(30)"`
		LockKey		int64		`xorm:"pk"`
		Counter		int64		`xorm:"NOT NULL DEFAULT 0"`
	}
	
	return x.Sync2(new(LockedResource))
}
