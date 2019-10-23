// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

func removeIndexColumnFromRepoUnitTable(x *xorm.Engine) (err error) {
	switch {
	case setting.Database.UseSQLite3:
		log.Warn("Unable to drop columns in SQLite")
	case setting.Database.UseMySQL, setting.Database.UsePostgreSQL, setting.Database.UseMSSQL:
		if _, err := x.Exec("ALTER TABLE repo_unit DROP COLUMN `index`"); err != nil {
			// Ignoring this error in case we run this migration second time (after migration reordering)
			log.Warn("DROP COLUMN index: %v", err)
		}
	default:
		log.Fatal("Unrecognized DB")
	}

	return nil
}
