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
// Cast the search value and the database column value to the same case for case-insensitive matching.
// * SQLite: only cast ASCII chars because it doesn't handle complete Unicode case folding
// * Other databases: use database's string function, assuming that they are able to handle complete Unicode case folding correctly
func BuildCaseInsensitiveLike(key, value string) builder.Cond {
	// ToLowerASCII is about 7% faster than ToUpperASCII (according to Golang's benchmark)
	if setting.Database.Type.IsSQLite3() {
		return builder.Like{"LOWER(" + key + ")", util.ToLowerASCII(value)}
	}
	return builder.Like{"LOWER(" + key + ")", strings.ToLower(value)}
}

// BuildCaseInsensitiveIn returns a condition to check if the given value is in the given values case-insensitively.
// See BuildCaseInsensitiveLike for more details
func BuildCaseInsensitiveIn(key string, values []string) builder.Cond {
	incaseValues := make([]string, len(values))
	caseCast := strings.ToLower
	if setting.Database.Type.IsSQLite3() {
		caseCast = util.ToLowerASCII
	}
	for i, value := range values {
		incaseValues[i] = caseCast(value)
	}
	return builder.In("LOWER("+key+")", incaseValues)
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
