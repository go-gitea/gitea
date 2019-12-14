// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/core"
	"xorm.io/xorm"
)

func changeReviewContentToText(x *xorm.Engine) error {

	if x.Dialect().DBType() == core.MYSQL {
		_, err := x.Exec("ALTER TABLE review MODIFY COLUMN content TEXT")
		return err
	}

	if x.Dialect().DBType() == core.ORACLE {
		_, err := x.Exec("ALTER TABLE review MODIFY content TEXT")
		return err
	}

	if x.Dialect().DBType() == core.MSSQL {
		_, err := x.Exec("ALTER TABLE review ALTER COLUMN content TEXT")
		return err
	}

	if x.Dialect().DBType() == core.POSTGRES {
		_, err := x.Exec("ALTER TABLE review ALTER COLUMN content TYPE TEXT")
		return err
	}

	// SQLite doesn't support ALTER COLUMN, and it seem to already make String to _TEXT_ default so no migration needed
	return nil
}
