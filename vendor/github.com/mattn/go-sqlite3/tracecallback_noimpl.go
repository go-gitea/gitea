// +build !trace

package sqlite3

import "errors"

func (c *SQLiteConn) RegisterAggregator(name string, impl interface{}, pure bool) error {
	return errors.New("This feature is not implemented")
}
