// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_11

import (
	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

func ChangeReviewContentToText(x *xorm.Engine) error {
	switch x.Dialect().URI().DBType {
	case schemas.MYSQL:
		_, err := x.Exec("ALTER TABLE review MODIFY COLUMN content TEXT")
		return err
	case schemas.ORACLE:
		_, err := x.Exec("ALTER TABLE review MODIFY content TEXT")
		return err
	case schemas.MSSQL:
		_, err := x.Exec("ALTER TABLE review ALTER COLUMN content TEXT")
		return err
	case schemas.POSTGRES:
		_, err := x.Exec("ALTER TABLE review ALTER COLUMN content TYPE TEXT")
		return err
	default:
		// SQLite doesn't support ALTER COLUMN, and it seem to already make String to _TEXT_ default so no migration needed
		return nil
	}
}
