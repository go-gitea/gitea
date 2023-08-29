// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/robfig/cron/v3"
)

// ActionScheduleSpec represents a schedule spec of a workflow file
type ActionScheduleSpec struct {
	ID         int64
	RepoID     int64                  `xorm:"index"`
	Repo       *repo_model.Repository `xorm:"-"`
	ScheduleID int64                  `xorm:"index"`
	Schedule   *ActionSchedule        `xorm:"-"`

	// Next time the job will run, or the zero time if Cron has not been
	// started or this entry's schedule is unsatisfiable
	Next timeutil.TimeStamp `xorm:"index"`
	// Prev is the last time this job was run, or the zero time if never.
	Prev timeutil.TimeStamp
	Spec string

	Created timeutil.TimeStamp `xorm:"created"`
	Updated timeutil.TimeStamp `xorm:"updated"`
}

func (s *ActionScheduleSpec) Parse() (cron.Schedule, error) {
	return cronParser.Parse(s.Spec)
}

func init() {
	db.RegisterModel(new(ActionScheduleSpec))
}

func UpdateScheduleSpec(ctx context.Context, spec *ActionScheduleSpec, cols ...string) error {
	sess := db.GetEngine(ctx).ID(spec.ID)
	if len(cols) > 0 {
		sess.Cols(cols...)
	}
	_, err := sess.Update(spec)
	return err
}
