// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_24

import (
	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

func AddRepoUnitAnonymousAccessMode(x base.EngineMigration) error {
	type RepoUnit struct { //revive:disable-line:exported
		AnonymousAccessMode int `xorm:"NOT NULL DEFAULT 0"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains: true,
		IgnoreIndices:    true,
	}, new(RepoUnit))
	return err
}
