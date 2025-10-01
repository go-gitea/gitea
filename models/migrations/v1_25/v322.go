// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_25

import (
	"code.gitea.io/gitea/models/migrations/base"

	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

func ExtendCommentTreePathLength(x *xorm.Engine) error {
	dbType := x.Dialect().URI().DBType
	if dbType == schemas.SQLITE { // For SQLITE, varchar or char will always be represented as TEXT
		return nil
	}

	return base.ModifyColumn(x, "comment", &schemas.Column{
		Name: "tree_path",
		SQLType: schemas.SQLType{
			Name: "VARCHAR",
		},
		Length:         4000,
		Nullable:       true, // To keep compatible as nullable
		DefaultIsEmpty: true,
	})
}
