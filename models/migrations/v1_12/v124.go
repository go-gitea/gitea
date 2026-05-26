// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_12

import "gitea.dev/models/db"

func AddUserRepoMissingColumns(x db.EngineMigration) error {
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
