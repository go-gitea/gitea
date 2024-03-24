// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22 //nolint

import (
	"code.gitea.io/gitea/models/project"

	"xorm.io/builder"
	"xorm.io/xorm"
)

func CheckProjectColumnsConsistency(x *xorm.Engine) error {
	var projects []project.Project
	if err := x.SQL("SELECT DISTINCT `p`.`id`, `p`.`creator_id` FROM `project` `p` WHERE (SELECT COUNT(*) FROM `project_board` `pb` WHERE `pb`.`project_id` = `p`.`id` AND `pb`.`default` = ?) != 1", true).
		Find(&projects); err != nil {
		return err
	}

	var boardsToCreate []project.Board
	for _, p := range projects {
		var boards []project.Board
		if err := x.Where("project_id=? AND `default` = ?", p.ID, true).OrderBy("sorting").Find(&boards); err != nil {
			return err
		}

		if len(boards) == 0 {
			boardsToCreate = append(boardsToCreate, project.Board{
				ProjectID: p.ID,
				Default:   true,
				Title:     "Uncategorized",
				CreatorID: p.CreatorID,
			})
			continue
		}

		var boardsToUpdate []int64
		for id, b := range boards {
			if id > 0 {
				boardsToUpdate = append(boardsToUpdate, b.ID)
			}
		}

		if _, err := x.Where(builder.Eq{"project_id": p.ID}.And(builder.In("id", boardsToUpdate))).
			Cols("`default`").Update(&project.Board{Default: false}); err != nil {
			return err
		}
	}

	if len(boardsToCreate) > 0 {
		if _, err := x.Insert(boardsToCreate); err != nil {
			return err
		}
	}

	return nil
}
