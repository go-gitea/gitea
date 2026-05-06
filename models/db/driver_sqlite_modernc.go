//go:build !sqlite_mattn

// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// modernc driver is chosen as the default one (compared to mattn, ncruces)
// * mattn was used as default, but it requires CGO
// * the CI times are almost the same for these three (race detector must be disabled)
// * modernc increases the binary size about 2MB, ncruces increases about 7MB
// * compiling time: modernc is slightly slower than mattn, ncruces is the slowest

package db

import (
	"database/sql"
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/setting"

	"modernc.org/sqlite"
)

func init() {
	// this driver contains huge amount of Golang code, so it is much slower when "-race" check is enabled.
	setting.SupportedDatabaseTypes = append(setting.SupportedDatabaseTypes, "sqlite3")
	makeSQLiteConnStr = makeSQLiteConnStrModerncCCGO
	sql.Register("sqlite3", &sqlite.Driver{})
}

func makeSQLiteConnStrModerncCCGO(opts SQLiteConnStrOptions) (string, string, error) {
	var params []string
	params = append(params, fmt.Sprintf("_pragma=busy_timeout(%d)", opts.BusyTimeout))
	params = append(params, "_txlock=immediate")
	if opts.JournalMode != "" {
		params = append(params, fmt.Sprintf("_pragma=journal_mode(%s)", opts.JournalMode))
	}
	connStr := fmt.Sprintf("file:%s?%s", opts.FilePath, strings.Join(params, "&"))
	return "sqlite3", connStr, nil
}
