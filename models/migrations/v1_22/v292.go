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

	if err := sess.Begin(); err != nil {
		return err
	}

	limit := setting.Database.IterateBufferSize
	if limit <= 0 {
		limit = 50
	}

	start := 0

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
		if start%200 == 0 {
			if err := sess.Begin(); err != nil {
				return err
			}
		}

		var projects []*Project
		if err := sess.Select("project.id as id, project.creator_id, project_board.id as board_id").
			Join("LEFT", "project_board", "project_board.project_id = project.id AND project_board.default=?", true).
			Where("project_board.id is NULL OR project_board.id = 0").
			Limit(limit, start).
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

		start += len(projects)
		if (start > 0 && start%200 == 0) || len(projects) == 0 {
			if err := sess.Commit(); err != nil {
				return err
			}
		}

		if len(projects) == 0 {
			break
		}
	}

	return sess.Commit()
}
