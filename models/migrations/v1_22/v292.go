// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22 //nolint

import (
	"code.gitea.io/gitea/models/project"

	"xorm.io/xorm"
)

func AddDefaultProjectColumn(x *xorm.Engine) error {
	var projectsWithMissingDefault []project.Project
	x.SQL("SELECT DISTINCT `p`.`id`, `p`.`creator_id` FROM `project` `p` WHERE NOT EXISTS (SELECT 1 FROM `project_board` `pb` WHERE `pb`.`project_id` = `p`.`id` AND `pb`.`default` != 0);").
		Find(&projectsWithMissingDefault)

	var boards []project.Board
	for _, p := range projectsWithMissingDefault {
		boards = append(boards, project.Board{
			ProjectID: p.ID,
			Default:   true,
			Title:     "Backlog",
			CreatorID: p.CreatorID,
		})
	}

	if len(boards) > 0 {
		if _, err := x.Insert(boards); err != nil {
			return err
		}
	}

	return nil
}
