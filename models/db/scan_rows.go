// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"xorm.io/xorm"
)

// ScanRows scans each row into a new Bean and hands it to f.
//
// It owns the whole rows lifecycle. A Scan failure, the iteration error that
// only Rows.Err reports, and the Close error are all accounted for here, with
// the first error winning. Callers that iterate rows by hand have to remember
// all three, and Rows.Err is the one that goes missing: Rows.Next returns false
// both when the rows run out and when the read failed part-way, so a caller
// that skips it answers from a truncated result set and reports success.
func ScanRows[Bean any](rows *xorm.Rows, f func(bean *Bean) error) (err error) {
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()
	for rows.Next() {
		var bean Bean
		if scanErr := rows.Scan(&bean); scanErr != nil {
			return scanErr
		}
		if fErr := f(&bean); fErr != nil {
			return fErr
		}
	}
	return rows.Err()
}
