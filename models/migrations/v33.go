// Copyright 2017 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/go-xorm/xorm"
)

func removeActionColumns(x *xorm.Engine) error {
	switch {
	case setting.UseSQLite3:
		log.Warn("Unable to drop columns in SQLite")
	case setting.UseMySQL, setting.UsePostgreSQL, setting.UseMSSQL, setting.UseTiDB:
		if _, err := x.Exec("ALTER TABLE action DROP COLUMN act_user_name"); err != nil {
			return fmt.Errorf("DROP COLUMN act_user_name: %v", err)
		} else if _, err = x.Exec("ALTER TABLE action DROP COLUMN repo_user_name"); err != nil {
			return fmt.Errorf("DROP COLUMN repo_user_name: %v", err)
		} else if _, err = x.Exec("ALTER TABLE action DROP COLUMN repo_name"); err != nil {
			return fmt.Errorf("DROP COLUMN repo_name: %v", err)
		}
	default:
		log.Fatal(4, "Unrecognized DB")
	}
	return nil
}
