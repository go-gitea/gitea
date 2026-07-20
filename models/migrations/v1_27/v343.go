// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"gitea.dev/models/db"

	"xorm.io/xorm"
)

func AddWatchOptions(x db.EngineMigration) error {
	type Watch struct { //revive:disable-line:exported
		PullRequests bool `xorm:"NOT NULL DEFAULT true"`
		Issues       bool `xorm:"NOT NULL DEFAULT true"`
		Releases     bool `xorm:"NOT NULL DEFAULT true"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains: true,
		IgnoreIndices:    true,
	}, new(Watch))
	return err
}
