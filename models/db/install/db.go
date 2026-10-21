// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package install

import (
	"context"

	"gitea.dev/models/db"
	"gitea.dev/modules/setting"
)

// CheckDatabaseConnection checks the database connection
func CheckDatabaseConnection(ctx context.Context) error {
	// Firebird rejects a SELECT without a FROM clause, so the probe has to read
	// from its one-row system table there
	probeSQL := "SELECT 1"
	if setting.Database.Type.IsFirebird() {
		probeSQL = "SELECT 1 FROM rdb$database"
	}
	_, err := db.GetEngine(ctx).Exec(probeSQL)
	return err
}

// GetMigrationVersion gets the database migration version
func GetMigrationVersion(ctx context.Context) (int64, error) {
	var installedDbVersion int64
	x := db.GetEngine(ctx)
	exist, err := x.IsTableExist("version")
	if err != nil {
		return 0, err
	}
	if !exist {
		return 0, nil
	}
	_, err = x.Table("version").Cols("version").Get(&installedDbVersion)
	if err != nil {
		return 0, err
	}
	return installedDbVersion, nil
}

// HasPostInstallationUsers checks whether there are users after installation
func HasPostInstallationUsers(ctx context.Context) (bool, error) {
	x := db.GetEngine(ctx)
	exist, err := x.IsTableExist("user")
	if err != nil {
		return false, err
	}
	if !exist {
		return false, nil
	}

	return x.Table("user").Exist()
}
