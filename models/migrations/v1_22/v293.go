// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22 //nolint

import (
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

// CheckProjectColumnsConsistency ensures there is exactly one default board per project present
func CheckProjectColumnsConsistency(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()

	limit := setting.Database.IterateBufferSize
	if limit <= 0 {
		limit = 50
	}

	type Project struct {
		ID        int64
		CreatorID int64
		BoardID   int64
	}

	type ProjectBoard struct {
		ID      int64 `xorm:"pk autoincr"`
		Title   string
		Default bool   `xorm:"NOT NULL DEFAULT false"` // issues not assigned to a specific board will be assigned to this board
		Sorting int8   `xorm:"NOT NULL DEFAULT 0"`
		Color   string `xorm:"VARCHAR(7)"`

		ProjectID int64 `xorm:"INDEX NOT NULL"`
		CreatorID int64 `xorm:"NOT NULL"`

		CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
	}

	for {
		if err := sess.Begin(); err != nil {
			return err
		}

		// all these projects without defaults will be fixed in the same loop, so
		// we just need to always get projects without defaults until no such project
		var projects []*Project
		if err := sess.Select("project.id as id, project.creator_id, project_board.id as board_id").
			Join("LEFT", "project_board", "project_board.project_id = project.id AND project_board.`default`=?", true).
			Where("project_board.id is NULL OR project_board.id = 0").
			Limit(limit).
			Find(&projects); err != nil {
			return err
		}

		for _, p := range projects {
			if _, err := sess.Insert(ProjectBoard{
				ProjectID: p.ID,
				Default:   true,
				Title:     "Uncategorized",
				CreatorID: p.CreatorID,
			}); err != nil {
				return err
			}
		}
		if err := sess.Commit(); err != nil {
			return err
		}

		if len(projects) == 0 {
			break
		}
	}
	sess.Close()

	return removeDuplicatedBoardDefault(x)
}

func removeDuplicatedBoardDefault(x *xorm.Engine) error {
	type ProjectInfo struct {
		ProjectID  int64
		DefaultNum int
	}
	var projects []ProjectInfo
	if err := x.Select("project_id, count(*) AS default_num").
		Table("project_board").
		Where("`default` = ?", true).
		GroupBy("project_id").
		Having("count(*) > 1").
		Find(&projects); err != nil {
		return err
	}

	for _, project := range projects {
		if _, err := x.Where("project_id=?", project.ProjectID).
			Table("project_board").
			Limit(project.DefaultNum - 1).
			Update(map[string]bool{
				"`default`": false,
			}); err != nil {
			return err
		}
	}
	return nil
}
