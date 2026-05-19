//go:build sqlite

// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"fmt"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/setting"

	_ "github.com/mattn/go-sqlite3"
)

func init() {
	setting.SupportedDatabaseTypes = append(setting.SupportedDatabaseTypes, "sqlite3")
	makeSQLiteConnStr = makeSQLiteConnStrMattnCGO
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
	return "sqlite3", connStr, nil
}
