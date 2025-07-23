// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_25

import (
	"code.gitea.io/gitea/models/migrations/base"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

func FixReviewStateUpdatedFilesColumn(x *xorm.Engine) error {
	if setting.Database.Type == "sqlite3" {
		return nil // SQLite does not support modify column type and the text type is already sufficient
	}

	return base.ModifyColumn(x, "review_state", &schemas.Column{
		Name: "updated_files",
		SQLType: schemas.SQLType{
			Name: "LONGTEXT",
		},
		Length:         0,
		Nullable:       false,
		DefaultIsEmpty: true,
	})
}
