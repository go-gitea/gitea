// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"

	"xorm.io/xorm"
)

func addDefaultValueToUserProhibitLogin(x *xorm.Engine) (err error) {
	user := &models.User{
		ProhibitLogin: false,
	}

	if _, err := x.Where("`prohibit_login` IS NULL").Cols("prohibit_login").Update(user); err != nil {
		return err
	}

	dialect := x.Dialect().DriverName()

	switch dialect {
	case "mysql":
		_, err = x.Exec("ALTER TABLE user MODIFY `prohibit_login` tinyint(1) NOT NULL DEFAULT 0")
	case "postgres":
		_, err = x.Exec("ALTER TABLE \"user\" ALTER COLUMN `prohibit_login` SET NOT NULL, ALTER COLUMN `prohibit_login` SET DEFAULT false")
	case "mssql":
		// xorm already set DEFAULT 0 for data type BIT in mssql
		_, err = x.Exec(`ALTER TABLE [user] ALTER COLUMN "prohibit_login" BIT NOT NULL`)
	case "sqlite3":
	}

	if err != nil {
		// Ignoring this error in case we run this migration second time (after migration reordering)
		log.Warn("Error changing user prohibit_login column definition (skipping): %v", err)
	}

	return nil
}
