// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import "xorm.io/xorm/schemas"

// DumpDatabase dumps all data from database according the special database SQL syntax to file system.
func DumpDatabase(filePath, dbType string) error {
	var tbs []*schemas.Table
	for _, t := range registeredModels {
		t, err := xormEngine.TableInfo(t)
		if err != nil {
			return err
		}
		tbs = append(tbs, t)
	}

	type Version struct {
		ID      int64 `xorm:"pk autoincr"`
		Version int64
	}
	t, err := xormEngine.TableInfo(&Version{})
	if err != nil {
		return err
	}
	tbs = append(tbs, t)

	if dbType != "" {
		return xormEngine.DumpTablesToFile(tbs, filePath, schemas.DBType(dbType))
	}
	return xormEngine.DumpTablesToFile(tbs, filePath)
}
