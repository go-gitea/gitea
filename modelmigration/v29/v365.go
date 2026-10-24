// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

func AddSSHKeyOwnerIDToMirrors(_ context.Context, x base.EngineMigration) error {
	type Mirror struct {
		SSHKeyOwnerID int64 `xorm:"NOT NULL DEFAULT 0"`
	}
	type PushMirror struct {
		SSHKeyOwnerID int64 `xorm:"NOT NULL DEFAULT 0"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains:  true,
		IgnoreDropIndices: true,
	}, new(Mirror), new(PushMirror))
	return err
}
