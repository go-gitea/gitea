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
		_, err = sess.Exec("ALTER TABLE repository ADD COLUMN `avatar` TEXT NOT NULL DEFAULT ''")
	case "postgres":
		_, err = sess.Exec("ALTER TABLE repository ADD COLUMN \"avatar\" VARCHAR NOT NULL DEFAULT ''")
	case "tidb":
		_, err = sess.Exec("ALTER TABLE repository ADD `avatar` TEXT NOT NULL DEFAULT ''")
	case "mssql":
		_, err = sess.Exec("ALTER TABLE repository ADD \"avatar\" VARCHAR NOT NULL DEFAULT ''")
	case "sqlite3":
		_, err = sess.Exec("ALTER TABLE repository ADD COLUMN `avatar` TEXT DEFAULT ''")
	}

	if err != nil {
		return fmt.Errorf("Error changing mirror interval column type: %v", err)
	}

	return sess.Commit()
}
