//go:build sqlite_mattn && sqlite_unlock_notify

// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"fmt"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

func init() {
	registerSQLiteConnStrMaker(makeSQLiteConnStrMattnCGO)
}

func makeSQLiteConnStrMattnCGO(opts SQLiteConnStrOptions) (string, string, error) {
	var params []string
	params = append(params, "cache=shared")
	params = append(params, "mode=rwc")
	params = append(params, "_busy_timeout="+strconv.Itoa(opts.BusyTimeout))
	params = append(params, "_txlock=immediate")
	if opts.JournalMode != "" {
		params = append(params, "_journal_mode="+opts.JournalMode)
	}
	connStr := fmt.Sprintf("file:%s?%s", opts.FilePath, strings.Join(params, "&"))
	return sqlDriverSQLite3, connStr, nil
}
