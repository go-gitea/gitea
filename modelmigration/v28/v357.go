// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

// AddShowPrivateActivityUserColumn adds the show_private_activity column to user
func AddShowPrivateActivityUserColumn(_ context.Context, x base.EngineMigration) error {
	type User struct {
		ShowPrivateActivity bool `xorm:"NOT NULL DEFAULT false"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	}, new(User))
	return err
}
