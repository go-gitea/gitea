// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_15

import (
	"code.gitea.io/gitea/models/migrations/base"

	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

func ConvertAvatarURLToText(x *xorm.Engine) error {
	dbType := x.Dialect().URI().DBType
	if dbType == schemas.SQLITE { // For SQLITE, varchar or char will always be represented as TEXT
		return nil
	}

	// Some oauth2 providers may give very long avatar urls (i.e. Google)
	return base.ModifyColumn(x, "external_login_user", &schemas.Column{
		Name: "avatar_url",
		SQLType: schemas.SQLType{
			Name: schemas.Text,
		},
		Nullable:       true,
		DefaultIsEmpty: true,
	})
}
