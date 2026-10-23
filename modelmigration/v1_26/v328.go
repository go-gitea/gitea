// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

func AddTokenPermissionsToActionRunJob(_ context.Context, x base.EngineMigration) error {
	type ActionRunJob struct {
		TokenPermissions string `xorm:"JSON TEXT"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true}, new(ActionRunJob))
	return err
}
