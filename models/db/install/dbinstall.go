// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package install

import (
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/setting"
)

// CheckDatabaseConnection checks the database connection
func CheckDatabaseConnection() error {
	e := db.GetEngine(db.DefaultContext)
	_, err := e.Exec("SELECT 1")
	return err
}

// GetMigrationVersion gets the database migration version
func GetMigrationVersion() int64 {
	var installedDbVersion int64
	e := db.GetEngine(db.DefaultContext)
	// the error can be safely ignored, then we still get version=0
	_, _ = e.Table("version").Cols("`version`").Get(&installedDbVersion)
	return installedDbVersion
}

// HasPostInstallationUsers checks whether there are users after installation
func HasPostInstallationUsers() bool {
	e := db.GetEngine(db.DefaultContext)
	// the error can be ignored safely, if there is no user table, we still get count=0
	// if there are 2 or more users in database, we consider there are users created after installation
	threshold := 2
	if !setting.IsProd {
		// to debug easily, with non-prod RUN_MODE, we only check the count to 1
		threshold = 1
	}
	res, _ := e.Table("user").Cols("id").Where("1=1").Limit(threshold).Query()
	return len(res) >= threshold
}
