// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"strings"
	"time"

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

// Parse parses the spec and returns a cron.Schedule
// Unlike the default cron parser, Parse uses UTC timezone as the default if none is specified.
func (s *ActionScheduleSpec) Parse() (cron.Schedule, error) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	schedule, err := parser.Parse(s.Spec)
	if err != nil {
		return nil, err
	}

	// If the spec has specified a timezone, use it
	if strings.HasPrefix(s.Spec, "TZ=") || strings.HasPrefix(s.Spec, "CRON_TZ=") {
		return schedule, nil
	}

	specSchedule, ok := schedule.(*cron.SpecSchedule)
	// If it's not a spec schedule, like "@every 5m", timezone is not relevant
	if !ok {
		return schedule, nil
	}

	// Set the timezone to UTC
	specSchedule.Location = time.UTC
	return specSchedule, nil
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
