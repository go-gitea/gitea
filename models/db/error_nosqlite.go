//go:build !sqlite

// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

// isErrDuplicateKeySQLite is a stub for non-SQLite builds.
// This function is only compiled when the sqlite build tag is NOT enabled.
func isErrDuplicateKeySQLite(err error) bool {
	return false
}

