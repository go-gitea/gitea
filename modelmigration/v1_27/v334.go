// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

func AddCancellingSupportToActionRunner(x base.EngineMigration) error {
	type ActionRunner struct {
		HasCancellingSupport bool `xorm:"has_cancelling_support NOT NULL DEFAULT false"`
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains:  true,
		IgnoreDropIndices: true,
	}, new(ActionRunner))
	return err
}
