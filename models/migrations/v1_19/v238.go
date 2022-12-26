// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_19 //nolint

import (
	"code.gitea.io/gitea/models/migrations/base"
	"xorm.io/xorm"
)

func RenameProjectsToBoards(x *xorm.Engine) error {
	if err := base.RenameTable(x, "project_board", "board_column"); err != nil {
		return err
	}
	if err := base.RenameTable(x, "project_issue", "board_issue"); err != nil {
		return err
	}
	if err := base.RenameTable(x, "project", "board"); err != nil {
		return err
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := base.RenameColumn(sess, "board_column", "project_id", "board_id", "bigint"); err != nil {
		return err
	}
	if err := base.RenameColumn(sess, "board_column", "project_id", "board_id", "bigint"); err != nil {
		return err
	}
	if err := base.RenameColumn(sess, "board_issue", "project_id", "board_id", "bigint"); err != nil {
		return err
	}
	if err := base.RenameColumn(sess, "board_issue", "project_board_id", "board_column_id", "bigint"); err != nil {
		return err
	}
	if err := base.RenameColumn(sess, "repository", "num_projects", "num_boards", "INT(11)"); err != nil {
		return err
	}
	if err := base.RenameColumn(sess, "repository", "num_closed_projects", "num_closed_boards", "INT(11)"); err != nil {
		return err
	}
	return nil
}
