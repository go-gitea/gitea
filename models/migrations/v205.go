// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

func migrateUserPasswordSalt(x *xorm.Engine) error {
	dbType := x.Dialect().URI().DBType
	// For SQLITE, the max length doesn't matter.
	if dbType == schemas.SQLITE {
		return nil
	}

	if err := modifyColumn(x, "user", &schemas.Column{
		Name: "rands",
		SQLType: schemas.SQLType{
			Name: "VARCHAR",
		},
		Length: 32,
		// MySQL will like us again.
		Nullable: true,
	}); err != nil {
		return err
	}

	return modifyColumn(x, "user", &schemas.Column{
		Name: "salt",
		SQLType: schemas.SQLType{
			Name: "VARCHAR",
		},
		Length:   32,
		Nullable: true,
	})
}
