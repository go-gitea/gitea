// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"code.gitea.io/gitea/models"

	"xorm.io/xorm"
)

func addIssueWatchModes(x *xorm.Engine) error {
	type IssueWatch struct {
		Mode models.IssueWatchMode `xorm:"SMALLINT NOT NULL DEFAULT 1"`
	}

	if err := x.Sync2(new(IssueWatch)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if _, err := sess.Where("is_watching = ?", false).Cols("mode").Update(&models.IssueWatch{Mode: models.IssueWatchModeDont}); err != nil {
		return err
	}
	if _, err := sess.Where("is_watching = ?", true).Cols("mode").Update(&models.IssueWatch{Mode: models.IssueWatchModeNormal}); err != nil {
		return err
	}

	// Drop column `is_watching`
	if _, err := sess.Exec("ALTER TABLE issue_watch DROP is_watching"); err != nil {
		return err
	}

	return nil
}
