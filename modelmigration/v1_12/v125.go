// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_12

import (
	"context"
	"fmt"

	"gitea.dev/modelmigration/base"
)

func AddReviewMigrateInfo(_ context.Context, x base.EngineMigration) error {
	type Review struct {
		OriginalAuthor   string
		OriginalAuthorID int64
	}

	if err := x.Sync(new(Review)); err != nil {
		return fmt.Errorf("Sync: %w", err)
	}
	return nil
}
