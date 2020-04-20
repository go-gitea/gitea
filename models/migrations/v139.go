// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/modules/timeutil"
	"xorm.io/xorm"
	"xorm.io/xorm/convert"
)

func addProjectsInfo(x *xorm.Engine) error {

	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	type (
		ProjectType      uint8
		ProjectBoardType uint8
	)

	type Project struct {
		ID              int64  `xorm:"pk autoincr"`
		Title           string `xorm:"INDEX NOT NULL"`
		Description     string `xorm:"TEXT"`
		RepoID          int64  `xorm:"NOT NULL"`
		CreatorID       int64  `xorm:"NOT NULL"`
		IsClosed        bool   `xorm:"INDEX"`
		NumIssues       int
		NumClosedIssues int

		BoardType ProjectBoardType
		Type      ProjectType

		ClosedDateUnix timeutil.TimeStamp
		CreatedUnix    timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix    timeutil.TimeStamp `xorm:"INDEX updated"`
	}

	if err := sess.Sync2(new(Project)); err != nil {
		return err
	}

	type Comment struct {
		OldProjectID int64
		ProjectID    int64
	}

	if err := sess.Sync2(new(Comment)); err != nil {
		return err
	}

	type Repository struct {
		ID                int64
		NumProjects       int `xorm:"NOT NULL DEFAULT 0"`
		NumClosedProjects int `xorm:"NOT NULL DEFAULT 0"`
		NumOpenProjects   int `xorm:"-"`
	}

	if err := sess.Sync2(new(Repository)); err != nil {
		return err
	}

	type Issue struct {
		ProjectID      int64 `xorm:"INDEX"`
		ProjectBoardID int64 `xorm:"INDEX"`
	}

	if err := sess.Sync2(new(Issue)); err != nil {
		return err
	}

	type ProjectBoard struct {
		ID        int64 `xorm:"pk autoincr"`
		ProjectID int64 `xorm:"INDEX NOT NULL"`
		Title     string
		RepoID    int64 `xorm:"INDEX NOT NULL"`

		CreatorID int64 `xorm:"NOT NULL"`

		CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
	}

	if err := sess.Sync2(new(ProjectBoard)); err != nil {
		return err
	}

	type RepoUnit struct {
		ID          int64
		RepoID      int64              `xorm:"INDEX(s)"`
		Type        int                `xorm:"INDEX(s)"`
		Config      convert.Conversion `xorm:"TEXT"`
		CreatedUnix timeutil.TimeStamp `xorm:"INDEX CREATED"`
	}

	const batchSize = 100

	const unitTypeProject int = 8 // see UnitTypeProjects in models/units.go

	for start := 0; ; start += batchSize {
		repos := make([]*Repository, 0, batchSize)

		if err := sess.Limit(batchSize, start).Find(&repos); err != nil {
			return err
		}

		if len(repos) == 0 {
			break
		}

		for _, r := range repos {
			if _, err := sess.ID(r.ID).Insert(&RepoUnit{
				RepoID:      r.ID,
				Type:        unitTypeProject,
				CreatedUnix: timeutil.TimeStampNow(),
			}); err != nil {
				return err
			}
		}
	}

	return sess.Commit()
}
