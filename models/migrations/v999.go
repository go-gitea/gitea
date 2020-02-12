// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/core"
	"xorm.io/xorm"
)

func addIssueWatchModes(x *xorm.Engine) error {
	type IssueWatch struct {
		ID          int64                 `xorm:"pk autoincr"`
		UserID      int64                 `xorm:"UNIQUE(watch) NOT NULL"`
		IssueID     int64                 `xorm:"UNIQUE(watch) NOT NULL"`
		Mode        models.IssueWatchMode `xorm:"NOT NULL DEFAULT 1"`
		CreatedUnix timeutil.TimeStamp    `xorm:"created NOT NULL"`
		UpdatedUnix timeutil.TimeStamp    `xorm:"updated NOT NULL"`
		//since it it is not used anymore and has NOT NULL constrain
		//it is altered to have a default value - we can drop it later ...
		IsWatching bool `xorm:"DEFAULT NULL"`
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if x.Dialect().DBType() == core.SQLITE {
		if _, err := sess.Exec("ALTER TABLE `issue_watch` RENAME TO `issue_watch_old`;"); err != nil {
			return err
		}
	}
	if err := x.Sync2(new(IssueWatch)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}
	if x.Dialect().DBType() == core.SQLITE {
		if _, err := sess.Exec("INSERT INTO `issue_watch` SELECT * FROM `issue_watch_old`;"); err != nil {
			return err
		}
		if err := sess.DropTable("issue_watch_old"); err != nil {
			return err
		}
	}

	if _, err := sess.Where("is_watching = ?", false).Cols("mode").Update(&models.IssueWatch{Mode: models.IssueWatchModeDont}); err != nil {
		return err
	}
	if _, err := sess.Where("is_watching = ?", true).Cols("mode").Update(&models.IssueWatch{Mode: models.IssueWatchModeNormal}); err != nil {
		return err
	}

	//add default value as suggested in: https://www.w3schools.com/sql/sql_default.asp
	//sqlite is done from L36-50 (you cant alter a column)
	switch x.Dialect().DBType() {
	case core.POSTGRES:
	case core.MYSQL:
		if _, err := sess.Exec("ALTER TABLE `issue_watch` MODIFY `is_watching` tinyint(1) NULL;"); err != nil {
			return err
		}
	case core.MSSQL:
		if _, err := sess.Exec("ALTER TABLE `issue_watch` ALTER COLUMN `is_watching` NULL;"); err != nil {
			return err
		}
	case core.ORACLE:
		if _, err := sess.Exec("ALTER TABLE issue_watch MODIFY is_watching NULL;"); err != nil {
			return err
		}
	}

	return nil
}
