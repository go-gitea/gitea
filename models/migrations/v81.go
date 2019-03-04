// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"github.com/go-xorm/xorm"
)

func changeU2FCounterType(x *xorm.Engine) error {
	type U2FRegistration struct {
		Counter uint32 `xorm:"BIGINT"`
	}

	var err error

	dialect := x.Dialect().DriverName()

	switch dialect {
	case "mysql":
		_, err = x.Exec("ALTER TABLE u2f_registration MODIFY `counter` BIGINT")
	case "postgres":
		_, err = x.Exec("ALTER TABLE u2f_registration ALTER COLUMN \"counter\" SET DATA TYPE bigint")
	case "tidb":
		_, err = x.Exec("ALTER TABLE u2f_registration MODIFY `counter` BIGINT")
	case "mssql":
		_, err = x.Exec("ALTER TABLE u2f_registration ALTER COLUMN \"counter\" BIGINT")
	case "sqlite3":
	}

	if err != nil {
		return fmt.Errorf("Error changing u2f_registration counter column type: %v", err)
	}

	return nil
}
