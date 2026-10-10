// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

func AddRunnerNameToActionTask(_ context.Context, x base.EngineMigration) error {
	type ActionTask struct {
		RunnerName string `xorm:"VARCHAR(255)"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	}, new(ActionTask))
	return err
}
