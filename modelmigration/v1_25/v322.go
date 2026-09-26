// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_25

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm/schemas"
)

func ExtendCommentTreePathLength(ctx context.Context, x base.EngineMigration) error {
	dbType := x.Dialect().URI().DBType
	if dbType == schemas.SQLITE { // For SQLITE, varchar or char will always be represented as TEXT
		return nil
	}

	return base.ModifyColumn(ctx, x, "comment", &schemas.Column{
		Name: "tree_path",
		SQLType: schemas.SQLType{
			Name: "VARCHAR",
		},
		Length:         4000,
		Nullable:       true, // To keep compatible as nullable
		DefaultIsEmpty: true,
	})
}
