// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func renameProjectBoardsToColumns(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if _, err := sess.Exec("ALTER TABLE `project_board` RENAME TO `project_column`;"); err != nil {
		return err
	}

	if _, err := sess.Exec("ALTER TABLE `project_issue` RENAME COLUMN `project_board_id` TO `project_column_id`;"); err != nil {
		return err
	}

	return sess.Commit()
}
