// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_28

import (
	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

func AddShowPrivateActivityUserColumn(x base.EngineMigration) error {
	type User struct {
		ShowPrivateActivity bool `xorm:"NOT NULL DEFAULT false"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	}, new(User))
	return err
}
