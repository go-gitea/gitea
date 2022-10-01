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

	if err := renameTable(sess, "project_board", "project_column"); err != nil {
		return err
	}

	if err := renameColumn(sess, "project_issue", "project_board_id", "project_column_id"); err != nil {
		return err
	}

	return sess.Commit()
}
