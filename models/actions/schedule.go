// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"
	"time"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"
	webhook_module "gitea.dev/modules/webhook"
)

// ActionSchedule represents a schedule of a workflow file
type ActionSchedule struct {
	ID            int64
	Title         string
	Specs         []string
	RepoID        int64                  `xorm:"index"`
	Repo          *repo_model.Repository `xorm:"-"`
	OwnerID       int64                  `xorm:"index"`
	WorkflowID    string
	TriggerUserID int64
	TriggerUser   *user_model.User `xorm:"-"`
	Ref           string
	CommitSHA     string
	Event         webhook_module.HookEventType
	EventPayload  string             `xorm:"LONGTEXT"`
	Content       []byte             `xorm:"LONGBLOB"`
	Created       timeutil.TimeStamp `xorm:"created"`
	Updated       timeutil.TimeStamp `xorm:"updated"`
}

func init() {
	db.RegisterModel(new(ActionSchedule))
}

// CreateScheduleTaskBySchedules creates new schedule task.
func CreateScheduleTaskBySchedules(ctx context.Context, rows []*ActionSchedule) error {
	// Return early if there are no rows to insert
	if len(rows) == 0 {
		return nil
	}

	return db.WithTx(ctx, func(ctx context.Context) error {
		// Loop through each schedule row
		for _, row := range rows {
			row.Title = util.EllipsisDisplayString(row.Title, 255)
			// Create new schedule row
			if err := db.Insert(ctx, row); err != nil {
				return err
			}

			// Loop through each schedule spec and create a new spec row
			now := time.Now()

			for _, spec := range row.Specs {
				specRow := &ActionScheduleSpec{
					RepoID:     row.RepoID,
					ScheduleID: row.ID,
					Spec:       spec,
				}
				schedule, err := specRow.Parse()
				if err != nil {
					continue
				}

				next := schedule.Next(now)
				if next.IsZero() {
					continue // the spec parses but can never occur, like "0 0 30 2 *"
				}
				specRow.Next = timeutil.TimeStamp(next.Unix())

				// Insert the new schedule spec row
				if err = db.Insert(ctx, specRow); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func DeleteScheduleTaskByRepo(ctx context.Context, id int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		if _, err := db.GetEngine(ctx).Delete(&ActionSchedule{RepoID: id}); err != nil {
			return err
		}

		if _, err := db.GetEngine(ctx).Delete(&ActionScheduleSpec{RepoID: id}); err != nil {
			return err
		}

		return nil
	})
}

func CleanRepoScheduleTasks(ctx context.Context, repo *repo_model.Repository) ([]*ActionRunJob, error) {
	// If actions disabled when there is schedule task, this will remove the outdated schedule tasks
	// There is no other place we can do this because the app.ini will be changed manually
	if err := DeleteScheduleTaskByRepo(ctx, repo.ID); err != nil {
		return nil, fmt.Errorf("DeleteCronTaskByRepo: %v", err)
	}
	// cancel running cron jobs of this repository and delete old schedules
	jobs, err := CancelPreviousJobs(
		ctx,
		repo.ID,
		repo.DefaultBranch,
		"",
		webhook_module.HookEventSchedule,
	)
	if err != nil {
		return jobs, fmt.Errorf("CancelPreviousJobs: %v", err)
	}
	return jobs, nil
}
