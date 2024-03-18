// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22 //nolint

import (
	"code.gitea.io/gitea/models/project"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func AddDefaultProjectColumn(x *xorm.Engine) error {
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
		var projects []*project.Project
		if err := sess.Select("id, creator_id").Limit(limit, start).Find(&projects); err != nil {
			return err
		}

		if len(projects) == 0 {
			break
		}
		start += len(projects)

		for _, p := range projects {
			var board project.Board
			exist, err := sess.Where("project_id=? AND `default`=?", p.ID, true).Get(&board)
			if err != nil {
				return err
			}

			if !exist {
				b := project.Board{
					CreatedUnix: timeutil.TimeStampNow(),
					CreatorID:   p.CreatorID,
					Title:       "Backlog",
					ProjectID:   p.ID,
					Default:     true,
				}
				if _, err := sess.Insert(b); err != nil {
					return err
				}
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
