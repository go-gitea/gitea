// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_8

import "gitea.dev/models/db"

func AddUserDefaultTheme(x db.EngineMigration) error {
	type User struct {
		Theme string `xorm:"VARCHAR(30) NOT NULL DEFAULT ''"`
	}

	return x.Sync(new(User))
}
