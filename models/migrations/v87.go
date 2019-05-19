// Copyright 2017 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"github.com/go-xorm/xorm"
)

func addAvatarFieldToRepository(x *xorm.Engine) (err error) {

	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	dialect := x.Dialect().DriverName()

	switch dialect {
	case "mysql":
		_, err = sess.Exec("ALTER TABLE repository ADD COLUMN `avatar` TEXT")
	case "postgres":
		_, err = sess.Exec("ALTER TABLE repository ADD COLUMN \"avatar\" VARCHAR")
	case "tidb":
		_, err = sess.Exec("ALTER TABLE repository ADD `avatar` TEXT")
	case "mssql":
		_, err = sess.Exec("ALTER TABLE repository ADD \"avatar\" VARCHAR")
	case "sqlite3":
		_, err = sess.Exec("ALTER TABLE repository ADD COLUMN `avatar` TEXT")
	}

	if err != nil {
		return fmt.Errorf("Error changing mirror interval column type: %v", err)
	}

	return sess.Commit()
}
