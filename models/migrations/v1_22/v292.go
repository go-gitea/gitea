// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22 //nolint

import (
	"code.gitea.io/gitea/models/project"

	"xorm.io/builder"
	"xorm.io/xorm"
)

func AddDefaultProjectColumn(x *xorm.Engine) error {
	if err := unsetWrongDefaultColumns(x); err != nil {
		return err
	}

	if err := addDefaultProjectColumn(x); err != nil {
		return err
	}

	return nil
}

// ensure a project has only one default column
func unsetWrongDefaultColumns(x *xorm.Engine) error {
	var projectsWithMultipleDefaults []int64
	x.SQL("SELECT DISTINCT `p`.`id` FROM `project` `p` WHERE (SELECT COUNT(*) FROM `project_board` `pb` WHERE `pb`.`project_id` = `p`.`id` AND `pb`.`default` != ? HAVING COUNT(*) > 1) > 1", false).
		Find(&projectsWithMultipleDefaults)

	for _, p := range projectsWithMultipleDefaults {
		var boards []project.Board
		x.Where("project_id=? AND `default`=?", p, false).OrderBy("sorting").Find(&boards)
		var boardsToUpdate []int64
		for id, b := range boards {
			if id > 0 {
				boardsToUpdate = append(boardsToUpdate, b.ID)
			}
		}
		if _, err := x.Where(builder.Eq{"project_id": p}.And(builder.In("id", boardsToUpdate))).
			Cols("`default`").Update(&project.Board{Default: false}); err != nil {
			return err
		}
	}

	return nil
}

// ensure every project has a default column
func addDefaultProjectColumn(x *xorm.Engine) error {
	var projectsWithMissingDefault []project.Project
	x.SQL("SELECT DISTINCT `p`.`id`, `p`.`creator_id` FROM `project` `p` WHERE NOT EXISTS (SELECT 1 FROM `project_board` `pb` WHERE `pb`.`project_id` = `p`.`id` AND `pb`.`default` != ?)", false).
		Find(&projectsWithMissingDefault)

	var boards []project.Board
	for _, p := range projectsWithMissingDefault {
		boards = append(boards, project.Board{
			ProjectID: p.ID,
			Default:   true,
			Title:     "Uncategorized",
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
