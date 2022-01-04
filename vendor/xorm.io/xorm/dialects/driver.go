// Copyright 2019 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dialects

import (
	"database/sql"
	"fmt"
	"time"

	"xorm.io/xorm/core"
)

// ScanContext represents a context when Scan
type ScanContext struct {
	DBLocation   *time.Location
	UserLocation *time.Location
}

// DriverFeatures represents driver feature
type DriverFeatures struct {
	SupportReturnInsertedID bool
}

// Driver represents a database driver
type Driver interface {
	Parse(string, string) (*URI, error)
	Features() *DriverFeatures
	GenScanResult(string) (interface{}, error) // according given column type generating a suitable scan interface
	Scan(*ScanContext, *core.Rows, []*sql.ColumnType, ...interface{}) error
}

var (
	drivers = map[string]Driver{}
)

// RegisterDriver register a driver
func RegisterDriver(driverName string, driver Driver) {
	if driver == nil {
		panic("core: Register driver is nil")
	}
	if _, dup := drivers[driverName]; dup {
		panic("core: Register called twice for driver " + driverName)
	}
	drivers[driverName] = driver
}

// QueryDriver query a driver with name
func QueryDriver(driverName string) Driver {
	return drivers[driverName]
}

// RegisteredDriverSize returned all drivers's length
func RegisteredDriverSize() int {
	return len(drivers)
}

// OpenDialect opens a dialect via driver name and connection string
func OpenDialect(driverName, connstr string) (Dialect, error) {
	driver := QueryDriver(driverName)
	if driver == nil {
		return nil, fmt.Errorf("unsupported driver name: %v", driverName)
	}

	uri, err := driver.Parse(driverName, connstr)
	if err != nil {
		return nil, err
	}

	dialect := QueryDialect(uri.DBType)
	if dialect == nil {
		return nil, fmt.Errorf("unsupported dialect type: %v", uri.DBType)
	}

	dialect.Init(uri)

	return dialect, nil
}

type baseDriver struct{}

func (b *baseDriver) Scan(ctx *ScanContext, rows *core.Rows, types []*sql.ColumnType, v ...interface{}) error {
	return rows.Scan(v...)
}
