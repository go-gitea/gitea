//go:build sqlite

// Copyright 2014 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	_ "github.com/mattn/go-sqlite3"
)

func init() {
	EnableSQLite3 = true
	SupportedDatabaseTypes = append(SupportedDatabaseTypes, "sqlite3")
}
