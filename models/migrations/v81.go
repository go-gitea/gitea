// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

func changeU2FCounterType(x *xorm.Engine) error {
	var err error

	switch x.Dialect().URI().DBType {
	case schemas.MYSQL:
		_, err = x.Exec("ALTER TABLE `u2f_registration` MODIFY `counter` BIGINT")
	case schemas.POSTGRES:
		_, err = x.Exec("ALTER TABLE `u2f_registration` ALTER COLUMN `counter` SET DATA TYPE bigint")
	case schemas.MSSQL:
		_, err = x.Exec("ALTER TABLE `u2f_registration` ALTER COLUMN `counter` BIGINT")
	}

	if err != nil {
		return fmt.Errorf("Error changing u2f_registration counter column type: %w", err)
	}

	return nil
}
