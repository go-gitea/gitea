// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_13 //nolint

import (
	"fmt"

	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

func IncreaseLanguageField(x *xorm.Engine) error {
	type LanguageStat struct {
		RepoID   int64  `xorm:"UNIQUE(s) INDEX NOT NULL"`
		Language string `xorm:"VARCHAR(50) UNIQUE(s) INDEX NOT NULL"`
	}

	if err := x.Sync(new(LanguageStat)); err != nil {
		return err
	}

	if setting.Database.Type.IsSQLite3() {
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
	case setting.Database.Type.IsMySQL():
		if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE language_stat MODIFY COLUMN language %s", sqlType)); err != nil {
			return err
		}
	case setting.Database.Type.IsMSSQL():
		// Yet again MSSQL just has to be awkward.
		// Here we have to drop the constraints first and then rebuild them
		constraints := make([]string, 0)
		if err := sess.SQL(`SELECT i.name AS Name
			FROM sys.indexes i INNER JOIN sys.index_columns ic
      			ON i.index_id = ic.index_id AND i.object_id = ic.object_id
   			INNER JOIN sys.tables AS t
      			ON t.object_id = i.object_id
			INNER JOIN sys.columns c
				ON t.object_id = c.object_id AND ic.column_id = c.column_id
			WHERE t.name = 'language_stat' AND c.name = 'language'`).Find(&constraints); err != nil {
			return fmt.Errorf("Find constraints: %w", err)
		}
		for _, constraint := range constraints {
			if _, err := sess.Exec(fmt.Sprintf("DROP INDEX [%s] ON `language_stat`", constraint)); err != nil {
				return fmt.Errorf("Drop table `language_stat` constraint `%s`: %w", constraint, err)
			}
		}
		if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE language_stat ALTER COLUMN language %s", sqlType)); err != nil {
			return err
		}
		// Finally restore the constraint
		if err := sess.CreateUniques(new(LanguageStat)); err != nil {
			return err
		}
	case setting.Database.Type.IsPostgreSQL():
		if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE language_stat ALTER COLUMN language TYPE %s", sqlType)); err != nil {
			return err
		}
	}

	return sess.Commit()
}
