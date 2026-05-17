// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22

import (
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/models/db"

)

func CreateAuthTokenTable(x db.EngineMigration) error {
	type AuthToken struct {
		ID          string `xorm:"pk"`
		TokenHash   string
		UserID      int64              `xorm:"INDEX"`
		ExpiresUnix timeutil.TimeStamp `xorm:"INDEX"`
	}

	return x.Sync(new(AuthToken))
}
