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
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if err := sess.Sync2(new(IssueWatch)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}

	if _, err := sess.Where("is_watching = ?", false).Cols("mode").Update(&models.IssueWatch{Mode: models.IssueWatchModeDont}); err != nil {
		return err
	}
	if _, err := sess.Where("is_watching = ?", true).Cols("mode").Update(&models.IssueWatch{Mode: models.IssueWatchModeNormal}); err != nil {
		return err
	}

	if err := sess.Commit(); err != nil {
		return err
	}

	switch x.Dialect().DBType() {
	case core.POSTGRES:
	case core.MYSQL:
	case core.MSSQL:
		if _, err := x.Exec("ALTER TABLE issue_watch DROP COLUMN is_watching;"); err != nil {
			return err
		}
	case core.SQLITE:
		if x.Dialect().DBType() == core.SQLITE {
			if _, err := x.Exec("CREATE TABLE temp.issue_watch_old AS SELECT * FROM issue_watch;"); err != nil {
				return err
			}
			if _, err := x.Exec("DROP TABLE issue_watch;"); err != nil {
				return err
			}

			if err := x.Sync2(new(IssueWatch)); err != nil {
				_, _ = x.Exec("CREATE TABLE issue_watch AS SELECT * FROM temp.issue_watch_old;")
				return fmt.Errorf("Sync2: %v", err)
			}

			if _, err := x.Exec("INSERT INTO `issue_watch` (user_id,issue_id,mode,created_unix,updated_unix) SELECT user_id,issue_id,mode,created_unix,updated_unix FROM temp.issue_watch_old;"); err != nil {
				return err
			}
			if _, err := x.Exec("DROP TABLE `temp.issue_watch_old`;"); err != nil {
				return err
			}
		}
	}
	return nil
}
