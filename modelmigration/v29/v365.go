// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

func AddMergeCommitSHAToActionRun(_ context.Context, x base.EngineMigration) error {
	type ActionRun struct {
		MergeCommitSHA string `xorm:"VARCHAR(64) NOT NULL DEFAULT ''"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains:  true,
		IgnoreDropIndices: true,
	}, new(ActionRun))
	return err
}
