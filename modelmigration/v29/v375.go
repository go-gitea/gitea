// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

func AddParentOrgIDToUser(_ctx context.Context, x base.EngineMigration) error {
	type User struct {
		ParentOrgID int64 `xorm:"parent_org_id INDEX NOT NULL DEFAULT 0"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains:  true,
		IgnoreDropIndices: true,
	}, new(User))
	return err
}
