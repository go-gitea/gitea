// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"strings"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// BuildCaseInsensitiveLike returns a condition to check if the given value is like the given key case-insensitively.
// Handles especially SQLite correctly as UPPER there only transforms ASCII letters.
func BuildCaseInsensitiveLike(key, value string) builder.Cond {
	if setting.Database.Type.IsSQLite3() {
		return builder.Like{"UPPER(" + key + ")", util.ToUpperASCII(value)}
	}
	return builder.Like{"UPPER(" + key + ")", strings.ToUpper(value)}
}

// BuilderDialect returns the xorm.Builder dialect of the engine
func BuilderDialect() string {
	switch {
	case setting.Database.Type.IsMySQL():
		return builder.MYSQL
	case setting.Database.Type.IsSQLite3():
		return builder.SQLITE
	case setting.Database.Type.IsPostgreSQL():
		return builder.POSTGRES
	case setting.Database.Type.IsMSSQL():
		return builder.MSSQL
	default:
		return ""
	}
}
