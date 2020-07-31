// Copyright (C) 2019 Yasuhiro Matsumoto <mattn.jp@gmail.com>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// +build !cgo

package sqlite3

import (
	"database/sql"
	"database/sql/driver"
	"errors"
)

func init() {
	sql.Register("sqlite3", &SQLiteDriverMock{})
}

type SQLiteDriverMock struct{}

var errorMsg = errors.New("Binary was compiled with 'CGO_ENABLED=0', go-sqlite3 requires cgo to work. This is a stub")

func (SQLiteDriverMock) Open(s string) (driver.Conn, error) {
	return nil, errorMsg
}
