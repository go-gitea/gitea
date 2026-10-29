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
	"gitea.dev/modules/git"
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

// ReplaceScheduleTasksByRepo replaces a repository's schedules with the ones given. A cron that is
// still registered for the same workflow keeps its existing spec row, and with it the scheduler's
// cursor: recomputing the cursor here would skip an occurrence that is due but not yet started.
func ReplaceScheduleTasksByRepo(ctx context.Context, repoID int64, rows []*ActionSchedule) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		registered, err := scheduleSpecsByCron(ctx, repoID)
		if err != nil {
			return err
		}
		if _, err := db.GetEngine(ctx).Delete(&ActionSchedule{RepoID: repoID}); err != nil {
			return err
		}

		var inUse []int64

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
				key := scheduleSpecKey{row.WorkflowID, spec}
				if specRow, ok := registered[key]; ok {
					// point the surviving row at the new schedule, writing nothing the scheduler owns
					delete(registered, key)
					specRow.ScheduleID = row.ID
					if err := UpdateScheduleSpec(ctx, specRow, "schedule_id"); err != nil {
						return err
					}
					inUse = append(inUse, specRow.ID)
					continue
				}

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
				inUse = append(inUse, specRow.ID)
			}
		}

		// drop the specs this registration did not claim, including any left orphaned earlier
		sess := db.GetEngine(ctx).Where("repo_id = ?", repoID)
		if len(inUse) > 0 {
			sess = sess.NotIn("id", inUse)
		}
		if _, err := sess.Delete(&ActionScheduleSpec{}); err != nil {
			return err
		}
		return nil
	})
}

type scheduleSpecKey struct {
	workflowID string
	spec       string
}

// scheduleSpecsByCron returns a repository's spec rows keyed by the workflow and cron they were
// registered for, so a replacement can tell which of them are still in use.
func scheduleSpecsByCron(ctx context.Context, repoID int64) (map[scheduleSpecKey]*ActionScheduleSpec, error) {
	schedules, err := db.Find[ActionSchedule](ctx, FindScheduleOptions{RepoID: repoID})
	if err != nil {
		return nil, fmt.Errorf("scheduleSpecsByCron: %w", err)
	}
	if len(schedules) == 0 {
		return map[scheduleSpecKey]*ActionScheduleSpec{}, nil
	}

	var specs []*ActionScheduleSpec
	if err := db.GetEngine(ctx).Where("repo_id = ?", repoID).Find(&specs); err != nil {
		return nil, fmt.Errorf("scheduleSpecsByCron: %w", err)
	}

	workflowIDs := make(map[int64]string, len(schedules))
	for _, schedule := range schedules {
		workflowIDs[schedule.ID] = schedule.WorkflowID
	}
	byCron := make(map[scheduleSpecKey]*ActionScheduleSpec, len(specs))
	for _, spec := range specs {
		if workflowID, ok := workflowIDs[spec.ScheduleID]; ok {
			byCron[scheduleSpecKey{workflowID, spec.Spec}] = spec
		}
	}
	return byCron, nil
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
		return nil, fmt.Errorf("CleanRepoScheduleTasks: %w", err)
	}
	// cancel running cron jobs of this repository and delete old schedules
	jobs, err := CancelPreviousJobs(
		ctx,
		repo.ID,
		git.RefNameFromBranch(repo.DefaultBranch).String(),
		"",
		webhook_module.HookEventSchedule,
	)
	if err != nil {
		return jobs, fmt.Errorf("CleanRepoScheduleTasks: %w", err)
	}
	return jobs, nil
}
