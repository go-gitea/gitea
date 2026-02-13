// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"strings"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// BuildCaseInsensitiveLike returns a case-insensitive LIKE condition for the given key and value.
// Cast the value and the database column value to the same cast to do case-insensitive matching.
// * SQLite: only cast ASCII chars because it doesn't handle complete Unicode case folding
// * Other databases: use database's string function, assuming that they are able to handle complete Unicode case folding correctly
// ToLowerASCII is about 7% faster than ToUpperASCII (according to Golang's benchmark)
func BuildCaseInsensitiveLike(key, value string) builder.Cond {
	if setting.Database.Type.IsSQLite3() {
		return builder.Like{"LOWER(" + key + ")", util.ToLowerASCII(value)}
	}
	return builder.Like{"LOWER(" + key + ")", strings.ToLower(value)}
}

// BuildCaseInsensitiveIn returns a condition to check if the given value is in the given values case-insensitively.
// See BuildCaseInsensitiveLike for more details
func BuildCaseInsensitiveIn(key string, values []string) builder.Cond {
	uppers := make([]string, len(values))
	transform := strings.ToLower
	if setting.Database.Type.IsSQLite3() {
		transform = util.ToLowerASCII
	}
	for i, value := range values {
		uppers[i] = transform(value)
	}

	return builder.In("LOWER("+key+")", uppers)
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
