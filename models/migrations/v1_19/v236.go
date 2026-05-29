// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_19

import (
	"gitea.dev/models/db"
	"gitea.dev/modules/timeutil"
)

func CreateSecretsTable(x db.EngineMigration) error {
	type Secret struct {
		ID          int64
		OwnerID     int64              `xorm:"INDEX UNIQUE(owner_repo_name) NOT NULL"`
		RepoID      int64              `xorm:"INDEX UNIQUE(owner_repo_name) NOT NULL DEFAULT 0"`
		Name        string             `xorm:"UNIQUE(owner_repo_name) NOT NULL"`
		Data        string             `xorm:"LONGTEXT"`
		CreatedUnix timeutil.TimeStamp `xorm:"created NOT NULL"`
	}

	return x.Sync(new(Secret))
}
