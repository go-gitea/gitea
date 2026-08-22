// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

func AddRecipientAccessGrantedToRepoTransfer(_ context.Context, x base.EngineMigration) error {
	type RepoTransfer struct {
		RecipientAccessGranted bool `xorm:"NOT NULL DEFAULT false"`
	}
	if _, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreConstrains:  true,
		IgnoreDropIndices: true,
	}, new(RepoTransfer)); err != nil {
		return err
	}

	const accessModeRead = 1
	_, err := x.Exec(`UPDATE repo_transfer SET recipient_access_granted = ?
		WHERE EXISTS (SELECT 1 FROM collaboration
			WHERE collaboration.repo_id = repo_transfer.repo_id
				AND collaboration.user_id = repo_transfer.recipient_id
				AND collaboration.mode = ?)`, true, accessModeRead)
	return err
}
