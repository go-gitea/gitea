// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"time"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/timeutil"
	webhook_module "code.gitea.io/gitea/modules/webhook"

	"github.com/robfig/cron/v3"
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
	EventPayload  string `xorm:"LONGTEXT"`
	Content       []byte
	Created       timeutil.TimeStamp `xorm:"created"`
	Updated       timeutil.TimeStamp `xorm:"updated"`
}

func init() {
	db.RegisterModel(new(ActionSchedule))
}

// GetSchedulesMapByIDs returns the schedules by given id slice.
func GetSchedulesMapByIDs(ctx context.Context, ids []int64) (map[int64]*ActionSchedule, error) {
	schedules := make(map[int64]*ActionSchedule, len(ids))
	return schedules, db.GetEngine(ctx).In("id", ids).Find(&schedules)
}

// GetReposMapByIDs returns the repos by given id slice.
func GetReposMapByIDs(ctx context.Context, ids []int64) (map[int64]*repo_model.Repository, error) {
	repos := make(map[int64]*repo_model.Repository, len(ids))
	return repos, db.GetEngine(ctx).In("id", ids).Find(&repos)
}

var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

// CreateScheduleTask creates new schedule task.
func CreateScheduleTask(ctx context.Context, rows []*ActionSchedule) error {
	// Return early if there are no rows to insert
	if len(rows) == 0 {
		return nil
	}

	// Begin transaction
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	// Loop through each schedule row
	for _, row := range rows {
		// Create new schedule row
		if err = db.Insert(ctx, row); err != nil {
			return err
		}

		// Loop through each schedule spec and create a new spec row
		now := time.Now()

		for _, spec := range row.Specs {
			// Parse the spec and check for errors
			schedule, err := cronParser.Parse(spec)
			if err != nil {
				continue // skip to the next spec if there's an error
			}

			// Insert the new schedule spec row
			if err = db.Insert(ctx, &ActionScheduleSpec{
				RepoID:     row.RepoID,
				ScheduleID: row.ID,
				Spec:       spec,
				Next:       timeutil.TimeStamp(schedule.Next(now).Unix()),
			}); err != nil {
				return err
			}
		}
	}

	// Commit transaction
	return committer.Commit()
}

func DeleteScheduleTaskByRepo(ctx context.Context, id int64) error {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()

	if _, err := db.GetEngine(ctx).Delete(&ActionSchedule{RepoID: id}); err != nil {
		return err
	}

	if _, err := db.GetEngine(ctx).Delete(&ActionScheduleSpec{RepoID: id}); err != nil {
		return err
	}

	return committer.Commit()
}
