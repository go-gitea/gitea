// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/timeutil"

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
		//for convert query's make sure column exist
		IsWatching bool
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

	if err := dropTableColumns(sess, "issue_watch", "is_watching"); err != nil {
		return err
	}

	return sess.Commit()
}
