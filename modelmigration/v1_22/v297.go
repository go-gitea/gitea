// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22

import (
	"context"

	"gitea.dev/modelmigration/base"
)

func AddRepoUnitEveryoneAccessMode(_ context.Context, x base.EngineMigration) error {
	type RepoUnit struct { //revive:disable-line:exported
		EveryoneAccessMode int `xorm:"NOT NULL DEFAULT 0"`
	}
	return x.Sync(&RepoUnit{})
}
