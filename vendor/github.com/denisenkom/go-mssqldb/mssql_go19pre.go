// +build !go1.9

package mssql

import (
	"database/sql/driver"
	"fmt"
)

func (s *Stmt) makeParamExtra(val driver.Value) (param, error) {
	return param{}, fmt.Errorf("mssql: unknown type for %T", val)
}
