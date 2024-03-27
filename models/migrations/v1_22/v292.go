// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22 //nolint

import (
	"code.gitea.io/gitea/models/project"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// CheckProjectColumnsConsistency ensures there is exactly one default board per project present
func CheckProjectColumnsConsistency(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	limit := setting.Database.IterateBufferSize
	if limit <= 0 {
		limit = 50
	}

	start := 0

	for {
		var projects []project.Project
		if err := sess.SQL("SELECT DISTINCT `p`.`id`, `p`.`creator_id` FROM `project` `p` WHERE (SELECT COUNT(*) FROM `project_board` `pb` WHERE `pb`.`project_id` = `p`.`id` AND `pb`.`default` = ?) != 1", true).
			Limit(limit, start).
			Find(&projects); err != nil {
			return err
		}

		if len(projects) == 0 {
			break
		}
		start += len(projects)

		for _, p := range projects {
			var boards []project.Board
			if err := sess.Where("project_id=? AND `default` = ?", p.ID, true).OrderBy("sorting").Find(&boards); err != nil {
				return err
			}

			if len(boards) == 0 {
				if _, err := sess.Insert(project.Board{
					ProjectID: p.ID,
					Default:   true,
					Title:     "Uncategorized",
					CreatorID: p.CreatorID,
				}); err != nil {
					return err
				}
				continue
			}

			var boardsToUpdate []int64
			for id, b := range boards {
				if id > 0 {
					boardsToUpdate = append(boardsToUpdate, b.ID)
				}
			}

			if _, err := sess.Where(builder.Eq{"project_id": p.ID}.And(builder.In("id", boardsToUpdate))).
				Cols("`default`").Update(&project.Board{Default: false}); err != nil {
				return err
			}
		}

		if start%1000 == 0 {
			if err := sess.Commit(); err != nil {
				return err
			}
			if err := sess.Begin(); err != nil {
				return err
			}
		}
	}

	return sess.Commit()
}
