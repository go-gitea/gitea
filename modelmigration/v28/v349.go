// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"context"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/setting"

	"xorm.io/xorm/schemas"
)

func ExpandActionScheduleContent(ctx context.Context, x base.EngineMigration) error {
	if !setting.Database.Type.IsMySQL() {
		return nil
	}

	return base.ModifyColumn(ctx, x, "action_schedule", &schemas.Column{
		Name: "content",
		SQLType: schemas.SQLType{
			Name: "LONGBLOB",
		},
		Length:         0,
		Nullable:       true,
		DefaultIsEmpty: true,
	})
}
