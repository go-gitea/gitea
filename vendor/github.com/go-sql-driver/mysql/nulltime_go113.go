// Go MySQL Driver - A MySQL-Driver for Go's database/sql package
//
// Copyright 2013 The Go-MySQL-Driver Authors. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at http://mozilla.org/MPL/2.0/.

// +build go1.13

package mysql

import (
	"database/sql"
)

// NullTime represents a time.Time that may be NULL.
// NullTime implements the Scanner interface so
// it can be used as a scan destination:
//
//  var nt NullTime
//  err := db.QueryRow("SELECT time FROM foo WHERE id=?", id).Scan(&nt)
//  ...
//  if nt.Valid {
//     // use nt.Time
//  } else {
//     // NULL value
//  }
//
// This NullTime implementation is not driver-specific
//
// Deprecated: NullTime doesn't honor the loc DSN parameter.
// NullTime.Scan interprets a time as UTC, not the loc DSN parameter.
// Use sql.NullTime instead.
type NullTime sql.NullTime

// for internal use.
// the mysql package uses sql.NullTime if it is available.
// if not, the package uses mysql.NullTime.
type nullTime = sql.NullTime // sql.NullTime is available
