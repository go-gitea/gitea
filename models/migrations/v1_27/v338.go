// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"gitea.dev/models/db"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"

	"xorm.io/xorm"
)

// AddActionRunJobMatchingSchema adds the schema runner task assignment needs to
// pick the oldest matchable waiting job efficiently in SQL:
//   - a normalized action_run_job_label table (one row per ActionRunJob.RunsOn
//     label) so labels can be matched in SQL instead of in memory, and
//   - a composite (status, updated) index so "oldest waiting job" is an index seek
//     instead of a sort of the whole waiting backlog.
func AddActionRunJobMatchingSchema(x db.EngineMigration) error {
	type ActionRunJobLabel struct {
		ID    int64  `xorm:"pk autoincr"`
		JobID int64  `xorm:"UNIQUE(job_label) NOT NULL"`
		Label string `xorm:"UNIQUE(job_label) INDEX VARCHAR(255) NOT NULL"`
	}

	if err := x.Sync(new(ActionRunJobLabel)); err != nil {
		return err
	}

	type ActionRunJob struct {
		ID      int64
		RunsOn  []string           `xorm:"JSON TEXT"`
		Status  int                `xorm:"index index(pick)"`
		Updated timeutil.TimeStamp `xorm:"index(pick)"`
	}

	// IgnoreDropIndices: only add the new composite (status, updated) index.
	if _, err := x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true}, new(ActionRunJob)); err != nil {
		return err
	}

	const (
		statusWaiting = 5 // actions.StatusWaiting
		statusBlocked = 7 // actions.StatusBlocked, becomes waiting once its needs complete
	)

	limit := setting.Database.IterateBufferSize
	if limit <= 0 {
		limit = 50
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	// Backfill labels for jobs that can still be assigned to a runner: those waiting
	// now and those blocked on dependencies (which become waiting later via an UPDATE
	// that wouldn't otherwise create rows). Finished/running jobs are never assigned
	// again, so they don't need rows. Page by id to keep the scan bounded.
	var lastID int64
	for {
		var jobs []ActionRunJob
		if err := sess.Where("status IN (?, ?) AND id > ?", statusWaiting, statusBlocked, lastID).
			OrderBy("id ASC").Limit(limit).Find(&jobs); err != nil {
			return err
		}
		if len(jobs) == 0 {
			break
		}

		var labels []ActionRunJobLabel
		for _, job := range jobs {
			lastID = job.ID
			seen := make(map[string]struct{}, len(job.RunsOn))
			for _, label := range job.RunsOn {
				if label == "" {
					continue
				}
				if _, ok := seen[label]; ok {
					continue
				}
				seen[label] = struct{}{}
				labels = append(labels, ActionRunJobLabel{JobID: job.ID, Label: label})
			}
		}

		if len(labels) > 0 {
			if _, err := sess.Insert(&labels); err != nil {
				return err
			}
		}

		if err := sess.Commit(); err != nil {
			return err
		}
		if err := sess.Begin(); err != nil {
			return err
		}
	}

	return sess.Commit()
}
