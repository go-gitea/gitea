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

	"modernc.org/sqlite"
)

func init() {
	// this driver contains huge amount of Golang code, so it is much slower when "-race" check is enabled.
	registerSQLiteConnStrMaker(makeSQLiteConnStrModerncCCGO)
	sql.Register(sqlDriverSQLite3, &sqlite.Driver{})
}

func makeSQLiteConnStrModerncCCGO(opts SQLiteConnStrOptions) (string, string, error) {
	var params []string
	// TODO: there is a changed behavior from mattn driver:
	// * mattn driver can wait for pretty long time for concurrent accesses (not limited by the busy timeout)
	// * but other drivers will report something like "database is locked (5) (SQLITE_BUSY)" if the timeout is reached
	// Maybe we need to relax the busy timeout to a reasonable long time in the future
	params = append(params, fmt.Sprintf("_pragma=busy_timeout(%d)", opts.BusyTimeout))
	params = append(params, "_txlock=immediate")
	if opts.JournalMode != "" {
		params = append(params, fmt.Sprintf("_pragma=journal_mode(%s)", opts.JournalMode))
	}
	connStr := fmt.Sprintf("file:%s?%s", opts.FilePath, strings.Join(params, "&"))
	return sqlDriverSQLite3, connStr, nil
}
