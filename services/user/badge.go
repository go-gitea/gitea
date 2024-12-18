// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
)

// UpdateBadgeDescription changes the description and/or image of a badge
func UpdateBadge(ctx context.Context, b *user_model.Badge) error {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err := user_model.UpdateBadge(ctx, b); err != nil {
		return err
	}
	return committer.Commit()
}

// DeleteBadge remove record of badge in the database
func DeleteBadge(ctx context.Context, b *user_model.Badge) error {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if err := user_model.DeleteBadge(ctx, b); err != nil {
		return fmt.Errorf("DeleteBadge: %w", err)
	}

	if err := committer.Commit(); err != nil {
		return err
	}
	_ = committer.Close()

	return nil
}
