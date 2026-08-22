// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

func AddRecipientCollaborationIDToRepoTransfer(_ context.Context, x base.EngineMigration) error {
	type RepoTransfer struct {
		RecipientCollaborationID int64 `xorm:"NOT NULL DEFAULT 0"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains:  true,
		IgnoreDropIndices: true,
	}, new(RepoTransfer))
	return err
}
