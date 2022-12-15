// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_19 //nolint

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func CreateSecretsTable(x *xorm.Engine) error {
	type Secret struct {
		ID          int64
		OwnerID     int64              `xorm:"INDEX UNIQUE(owner_repo_name) NOTNULL"`
		RepoID      int64              `xorm:"INDEX UNIQUE(owner_repo_name) NOTNULL"`
		Name        string             `xorm:"UNIQUE(owner_repo_name) NOTNULL"`
		Data        string             `xorm:"LONGTEXT"`
		CreatedUnix timeutil.TimeStamp `xorm:"created NOTNULL"`
	}

	return x.Sync(new(Secret))
}
