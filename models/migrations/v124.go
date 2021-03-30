// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func addUserRepoMissingColumns(x *xorm.Engine) error {
	type VisibleType int
	type User struct {
		PasswdHashAlgo string      `xorm:"NOT NULL DEFAULT 'pbkdf2'"`
		Visibility     VisibleType `xorm:"NOT NULL DEFAULT 0"`
	}

	type Repository struct {
		IsArchived bool     `xorm:"INDEX"`
		Topics     []string `xorm:"TEXT JSON"`
	}

	return x.Sync2(new(User), new(Repository))
}
