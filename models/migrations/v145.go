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
		// Yet again MSSQL just has to be awkward.
		// Here we have to drop the constraints first and then rebuild them
		constraints := make([]string, 0)
		if err := sess.SQL(
			"SELECT Name FROM SYS.DEFAULT_CONSTRAINTS WHERE " +
				"PARENT_OBJECT_ID = OBJECT_ID('language_stat') AND " +
				"PARENT_COLUMN_ID IN (SELECT column_id FROM sys.columns " +
				"WHERE lower(NAME) = 'language' AND " +
				"object_id = OBJECT_ID('language_stat'))").Find(&constraints); err != nil {
			return fmt.Errorf("Find constraints: %v", err)
		}
		for _, constraint := range constraints {
			if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE `language_stat` DROP CONSTRAINT `%s`", constraint)); err != nil {
				return fmt.Errorf("Drop table `language_stat` constraint `%s`: %v", constraint, err)
			}
		}
		if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE language_stat ALTER COLUMN language %s", sqlType)); err != nil {
			return err
		}
		// Finally restore the constraint
		if err := sess.Sync2(new(LanguageStat)); err != nil {
			return err
		}
	case setting.Database.UsePostgreSQL:
		if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE language_stat ALTER COLUMN language TYPE %s", sqlType)); err != nil {
			return err
		}
	}

	return sess.Commit()
}
