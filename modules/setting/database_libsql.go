//go:build libsql

// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	_ "github.com/libsql/go-libsql"
)

func init() {
	EnableLibSQL = true
	SupportedDatabaseTypes = append(SupportedDatabaseTypes, "libsql")
}
