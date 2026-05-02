//go:build sqlite_ncruces

// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/setting"

	_ "github.com/ncruces/go-sqlite3/driver"
)

func init() {
	setting.SupportedDatabaseTypes = append(setting.SupportedDatabaseTypes, "sqlite3")
	makeSQLiteConnStr = makeSQLiteConnStrNcrucesWasm
}

func makeSQLiteConnStrNcrucesWasm(opts SQLiteConnStrOptions) (string, string, error) {
	var params []string
	params = append(params, fmt.Sprintf("_pragma=busy_timeout(%d)", opts.BusyTimeout))
	params = append(params, "_txlock=immediate")
	if opts.JournalMode != "" {
		params = append(params, fmt.Sprintf("_pragma=journal_mode(%s)", strings.ToLower(opts.JournalMode)))
	}
	connStr := fmt.Sprintf("file:%s?%s", opts.FilePath, strings.Join(params, "&"))
	return "sqlite3", connStr, nil
}
