//go:build sqlite

// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"errors"

	"github.com/mattn/go-sqlite3"
)

// isErrDuplicateKeySQLite checks if an error is a SQLite unique constraint violation.
// This function is only compiled when the sqlite build tag is enabled.
func isErrDuplicateKeySQLite(err error) bool {
	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) {
		return sqliteErr.Code == sqlite3.ErrConstraint || sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique
	}
	return false
}

