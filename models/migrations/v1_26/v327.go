// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"gitea.dev/models/db"

	"xorm.io/xorm"
)

func AddDisabledToActionRunner(x db.EngineMigration) error {
	type ActionRunner struct {
		IsDisabled bool `xorm:"is_disabled NOT NULL DEFAULT false"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
	}, new(ActionRunner))
	return err
}
