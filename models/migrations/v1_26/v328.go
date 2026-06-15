// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"gitea.dev/models/db"

	"xorm.io/xorm"
)

func AddTokenPermissionsToActionRunJob(x db.EngineMigration) error {
	type ActionRunJob struct {
		TokenPermissions string `xorm:"JSON TEXT"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true}, new(ActionRunJob))
	return err
}
