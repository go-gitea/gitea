// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_10

import (
	"context"

	"gitea.dev/modelmigration/base"
)

func AddOriginalMigrationInfo(_ context.Context, x base.EngineMigration) error {
	type Issue struct {
		OriginalAuthor   string
		OriginalAuthorID int64
	}

	if err := x.Sync(new(Issue)); err != nil {
		return err
	}

	type Comment struct {
		OriginalAuthor   string
		OriginalAuthorID int64
	}

	if err := x.Sync(new(Comment)); err != nil {
		return err
	}

	type Repository struct {
		OriginalURL string
	}

	return x.Sync(new(Repository))
}
