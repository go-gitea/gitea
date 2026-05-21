// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22

import (
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
)

func AddRepoUnitEveryoneAccessMode(x db.EngineMigration) error {
	type RepoUnit struct { //revive:disable-line:exported
		EveryoneAccessMode perm.AccessMode `xorm:"NOT NULL DEFAULT 0"`
	}
	return x.Sync(&RepoUnit{})
}
