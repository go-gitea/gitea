// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

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
	if setting.Database.UseSQLite3 {
		return builder.Like{"UPPER(" + key + ")", util.ToUpperASCII(value)}
	}
	return builder.Like{"UPPER(" + key + ")", strings.ToUpper(value)}
}
