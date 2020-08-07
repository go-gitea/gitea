// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

func increaseLanguageField(x *xorm.Engine) error {
	type LanguageStat struct {
		RepoID   int64  `xorm:"UNIQUE(s) INDEX NOT NULL"`
		Language string `xorm:"VARCHAR(50) UNIQUE(s) INDEX NOT NULL"`
	}

	if err := x.Sync2(new(LanguageStat)); err != nil {
		return err
	}

	if setting.Database.UseSQLite3 {
		// SQLite maps VARCHAR to TEXT without size so we're done
		return nil
	}

	// need to get the correct type for the new column
	inferredTable, err := x.TableInfo(new(LanguageStat))
	if err != nil {
		return err
	}
	column := inferredTable.GetColumn("language")
	sqlType := x.Dialect().SQLType(column)

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	switch {
	case setting.Database.UseMySQL:
		if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE language_stat MODIFY COLUMN language %s", sqlType)); err != nil {
			return err
		}
	case setting.Database.UseMSSQL:
		if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE language_stat ALTER COLUMN language %s", sqlType)); err != nil {
			return err
		}
	case setting.Database.UsePostgreSQL:
		if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE language_stat ALTER COLUMN language TYPE %s", sqlType)); err != nil {
			return err
		}
	}

	return sess.Commit()
}
