// +build sqlite

// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import "code.gitea.io/gitea/traceinit"

import (
	_ "github.com/mattn/go-sqlite3"
)

func init() {
	traceinit.Trace("./modules/setting/database_sqlite.go")
	EnableSQLite3 = true
	SupportedDatabases = append(SupportedDatabases, "SQLite3")
}
