// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/go-xorm/xorm"
)

func removeIsOwnerColumnFromOrgUser(x *xorm.Engine) (err error) {
	switch {
	case setting.UseSQLite3:
		log.Warn("Unable to drop columns in SQLite")
	case setting.UseMySQL, setting.UseTiDB, setting.UsePostgreSQL:
		if _, err := x.Exec("ALTER TABLE org_user DROP COLUMN is_owner, DROP COLUMN num_teams"); err != nil {
			return fmt.Errorf("DROP COLUMN org_user.is_owner, org_user.num_teams: %v", err)
		}
	case setting.UseMSSQL:
		if _, err := x.Exec("ALTER TABLE org_user DROP COLUMN is_owner, num_teams"); err != nil {
			return fmt.Errorf("DROP COLUMN org_user.is_owner, org_user.num_teams: %v", err)
		}
	default:
		log.Fatal(4, "Unrecognized DB")
	}

	return nil
}
