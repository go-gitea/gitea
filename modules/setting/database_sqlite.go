//go:build sqlite

// Copyright 2014 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

// TODO: remove all "sqlite_unlock_notify" tag

func init() {
	EnableSQLite3 = true
	SupportedDatabaseTypes = append(SupportedDatabaseTypes, "sqlite3")
}
