// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
)

// RenameBadge changes the slug of a badge.
func RenameBadge(ctx context.Context, b *user_model.Badge, newSlug string) error {
	if newSlug == b.Slug {
		return nil
	}

	olderSlug := b.Slug

	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	isExist, err := user_model.IsBadgeExist(ctx, b.ID, newSlug)
	if err != nil {
		return err
	}
	if isExist {
		return user_model.ErrBadgeAlreadyExist{
			Slug: newSlug,
		}
	}

	b.Slug = newSlug
	if err := user_model.UpdateBadge(ctx, b); err != nil {
		b.Slug = olderSlug
		return err
	}
	if err = committer.Commit(); err != nil {
		b.Slug = olderSlug
		return err
	}
	return nil
}

// DeleteBadge completely and permanently deletes everything of a badge
func DeleteBadge(ctx context.Context, b *user_model.Badge, purge bool) error {
	if purge {
		err := user_model.DeleteUserBadgeRecord(ctx, b)
		if err != nil {
			return err
		}
	}

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
