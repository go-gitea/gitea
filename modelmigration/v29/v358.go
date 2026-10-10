// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

func AddStageAndNumberToActionTaskStep(_ context.Context, x base.EngineMigration) error {
	type ActionTaskStep struct {
		Stage  int32 `xorm:"NOT NULL DEFAULT 0"`
		Number int64 `xorm:"NOT NULL DEFAULT 0"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains:  true,
		IgnoreDropIndices: true,
	}, new(ActionTaskStep))
	return err
}
