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
		//it is altered to allow NULL - we can drop it later ...
		IsWatching bool
	}

	if x.Dialect().DBType() == core.SQLITE {
		_, _ = x.Exec("DROP TABLE `issue_watch_old`;")
		_, _ = x.Exec("DROP INDEX `UQE_issue_watch_watch`;")

		if _, err := x.Exec("ALTER TABLE `issue_watch` RENAME TO `issue_watch_old`;"); err != nil {
			return err
		}

		if err := x.Sync2(new(IssueWatch)); err != nil {
			_, _ = x.Exec("ALTER TABLE `issue_watch_old` RENAME TO `issue_watch`;")
			return fmt.Errorf("Sync2: %v", err)
		}

		if _, err := x.Exec("INSERT INTO `issue_watch` (user_id,issue_id,is_watching,created_unix,updated_unix) SELECT user_id,issue_id,is_watching,created_unix,updated_unix FROM `issue_watch_old`;"); err != nil {
			return err
		}
		if _, err := x.Exec("DROP TABLE `issue_watch_old`;"); err != nil {
			return err
		}
	}
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	if x.Dialect().DBType() != core.SQLITE {
		if err := sess.Sync2(new(IssueWatch)); err != nil {
			return fmt.Errorf("Sync2: %v", err)
		}
	}

	if _, err := sess.Where("is_watching = ?", false).Cols("mode").Update(&models.IssueWatch{Mode: models.IssueWatchModeDont}); err != nil {
		return err
	}
	if _, err := sess.Where("is_watching = ?", true).Cols("mode").Update(&models.IssueWatch{Mode: models.IssueWatchModeNormal}); err != nil {
		return err
	}

	//sqlite is done from L36-49 (you cant alter a column)
	switch x.Dialect().DBType() {
	case core.POSTGRES:
		if _, err := sess.Exec("ALTER TABLE issue_watch ALTER COLUMN is_watching DROP NOT NULL;"); err != nil {
			return err
		}
	case core.MSSQL:
		if _, err := sess.Exec("ALTER TABLE issue_watch ALTER COLUMN is_watching bit NULL;"); err != nil {
			return err
		}
	case core.MYSQL:
		if _, err := sess.Exec("ALTER TABLE `issue_watch` MODIFY `is_watching` tinyint(1) NULL;"); err != nil {
			return err
		}
	}

	return sess.Commit()
}
