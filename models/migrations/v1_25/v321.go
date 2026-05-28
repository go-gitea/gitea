// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_25

import (
	"gitea.dev/models/db"
	"gitea.dev/models/migrations/base"
	"gitea.dev/modules/setting"

	"xorm.io/xorm/schemas"
)

func UseLongTextInSomeColumnsAndFixBugs(x db.EngineMigration) error {
	if !setting.Database.Type.IsMySQL() {
		return nil // Only mysql need to change from text to long text, for other databases, they are the same
	}

	if err := base.ModifyColumn(x, "review_state", &schemas.Column{
		Name: "updated_files",
		SQLType: schemas.SQLType{
			Name: "LONGTEXT",
		},
		Length:         0,
		Nullable:       false,
		DefaultIsEmpty: true,
	}); err != nil {
		return err
	}

	if err := base.ModifyColumn(x, "package_property", &schemas.Column{
		Name: "value",
		SQLType: schemas.SQLType{
			Name: "LONGTEXT",
		},
		Length:         0,
		Nullable:       false,
		DefaultIsEmpty: true,
	}); err != nil {
		return err
	}

	return base.ModifyColumn(x, "notice", &schemas.Column{
		Name: "description",
		SQLType: schemas.SQLType{
			Name: "LONGTEXT",
		},
		Length:         0,
		Nullable:       false,
		DefaultIsEmpty: true,
	})
}
