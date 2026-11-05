// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

func AddUseOwnerSSHKeyToMirrors(_ context.Context, x base.EngineMigration) error {
	type Mirror struct {
		UseOwnerSSHKey bool `xorm:"NOT NULL DEFAULT false"`
	}
	type PushMirror struct {
		UseOwnerSSHKey bool `xorm:"NOT NULL DEFAULT false"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains:  true,
		IgnoreDropIndices: true,
	}, new(Mirror), new(PushMirror))
	return err
}
