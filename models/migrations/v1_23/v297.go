// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23 //nolint

import (
	"xorm.io/xorm"

	"code.gitea.io/gitea/models/perm"
)

func AddRepoUnitEveryoneAccessMode(x *xorm.Engine) error {
	type RepoUnit struct { //revive:disable-line:exported
		EveryoneAccessMode perm.AccessMode `xorm:"NOT NULL DEFAULT -1"`
	}
	return x.Sync(&RepoUnit{})
}
