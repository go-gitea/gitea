// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_7

import "gitea.dev/models/db"

func AddMustChangePassword(x db.EngineMigration) error {
	// User see models/user.go
	type User struct {
		ID                 int64 `xorm:"pk autoincr"`
		MustChangePassword bool  `xorm:"NOT NULL DEFAULT false"`
	}

	return x.Sync(new(User))
}
