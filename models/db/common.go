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

func BuildLikeUpper(key, value string) builder.Cond {
	if setting.Database.UseSQLite3 {
		// SQLite's UPPER function only transforms ASCII letters.
		return builder.Like{"UPPER(" + key + ")", util.ToUpperASCII(value)}
	} else {
		return builder.Like{"UPPER(" + key + ")", strings.ToUpper(value)}
	}
}
