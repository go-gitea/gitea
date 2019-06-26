// +build go1.10

package mssql

import (
	"database/sql/driver"
)

var _ driver.Connector = &Connector{}
var _ driver.SessionResetter = &Conn{}
