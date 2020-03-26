// Copyright 2019 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dialects

import (
	"fmt"
)

type Driver interface {
	Parse(string, string) (*URI, error)
}

var (
	drivers = map[string]Driver{}
)

func RegisterDriver(driverName string, driver Driver) {
	if driver == nil {
		panic("core: Register driver is nil")
	}
	if _, dup := drivers[driverName]; dup {
		panic("core: Register called twice for driver " + driverName)
	}
	drivers[driverName] = driver
}

func QueryDriver(driverName string) Driver {
	return drivers[driverName]
}

func RegisteredDriverSize() int {
	return len(drivers)
}

// OpenDialect opens a dialect via driver name and connection string
func OpenDialect(driverName, connstr string) (Dialect, error) {
	driver := QueryDriver(driverName)
	if driver == nil {
		return nil, fmt.Errorf("Unsupported driver name: %v", driverName)
	}

	uri, err := driver.Parse(driverName, connstr)
	if err != nil {
		return nil, err
	}

	dialect := QueryDialect(uri.DBType)
	if dialect == nil {
		return nil, fmt.Errorf("Unsupported dialect type: %v", uri.DBType)
	}

	dialect.Init(uri)

	return dialect, nil
}
