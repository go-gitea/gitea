// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

func AddDisabledToActionRunner(_ context.Context, x base.EngineMigration) error {
	type ActionRunner struct {
		IsDisabled bool `xorm:"is_disabled NOT NULL DEFAULT false"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
	}, new(ActionRunner))
	return err
}
