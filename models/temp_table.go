// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"sync/atomic"

	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

var tempTableSequence uint64

// createSimpleTemporaryTable will create a temporary table with the given
// name template and column definitions; it returns the actual name used
// and a cleanup function.
func createSimpleTemporaryTable(sess *xorm.Session, columns string) (string, func() error, error) {

	var name, create, drop string

	seq := atomic.AddUint64(&tempTableSequence, 1)

	switch {
	case setting.Database.UseSQLite3:
		name = fmt.Sprintf("temp.temp_table_%d", seq)
		create = fmt.Sprintf("CREATE TABLE %s (%s)", name, columns)
		drop = fmt.Sprintf("DROP TABLE %s", name)
	case setting.Database.UseMSSQL:
		name = fmt.Sprintf("#temp_table_%d", seq)
		create = fmt.Sprintf("CREATE TABLE %s (%s)", name, columns)
		drop = fmt.Sprintf("DROP TABLE %s", name)
	default:
		name = fmt.Sprintf("temp_table_%d", seq)
		create = fmt.Sprintf("CREATE TEMPORARY TABLE %s (%s)", name, columns)
		drop = fmt.Sprintf("DROP TEMPORARY TABLE %s", name)
	}
	_, err := sess.Exec(create)
	if err != nil {
		return "", func() error {
			return nil
		}, err
	}
	return name, func() (err error) {
		// Note: calling the cleanup function is optional
		// as the temporary table will be dropped when the connection
		// is reset for the next request.
		_, err = sess.Exec(drop)
		return
	}, nil
}
