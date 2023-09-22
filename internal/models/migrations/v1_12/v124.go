// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_12 //nolint

import (
	"xorm.io/xorm"
)

func AddUserRepoMissingColumns(x *xorm.Engine) error {
	type VisibleType int
	type User struct {
		PasswdHashAlgo string      `xorm:"NOT NULL DEFAULT 'pbkdf2'"`
		Visibility     VisibleType `xorm:"NOT NULL DEFAULT 0"`
	}

	type Repository struct {
		IsArchived bool     `xorm:"INDEX"`
		Topics     []string `xorm:"TEXT JSON"`
	}

	return x.Sync(new(User), new(Repository))
}
