// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_24

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

func AddEphemeralToActionRunner(_ context.Context, x base.EngineMigration) error {
	type ActionRunner struct {
		Ephemeral bool `xorm:"ephemeral NOT NULL DEFAULT false"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains: true,
		IgnoreIndices:    true,
	}, new(ActionRunner))
	return err
}
