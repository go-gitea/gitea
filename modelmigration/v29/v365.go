// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

func AddOriginalIDToCommentAndReview(_ context.Context, x base.EngineMigration) error {
	type Comment struct {
		OriginalID int64 `xorm:"index"`
	}
	type Review struct {
		OriginalID int64 `xorm:"index"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	}, new(Comment), new(Review))
	return err
}
