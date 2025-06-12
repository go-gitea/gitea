// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"
	webhook_module "code.gitea.io/gitea/modules/webhook"
	notify_service "code.gitea.io/gitea/services/notify"

	"github.com/nektos/act/pkg/jobparser"
)

// StartScheduleTasks start the task
func StartScheduleTasks(ctx context.Context) error {
	return startTasks(ctx)
}

// startTasks retrieves specifications in pages, creates a schedule task for each specification,
// and updates the specification's next run time and previous run time.
// The function returns an error if there's an issue with finding or updating the specifications.
func startTasks(ctx context.Context) error {
	// Set the page size
	pageSize := 50

	// Retrieve specs in pages until all specs have been retrieved
	now := time.Now()
	for page := 1; ; page++ {
		// Retrieve the specs for the current page
		specs, _, err := actions_model.FindSpecs(ctx, actions_model.FindSpecOptions{
			ListOptions: db.ListOptions{
				Page:     page,
				PageSize: pageSize,
			},
			Next: now.Unix(),
		})
		if err != nil {
			return fmt.Errorf("find specs: %w", err)
		}

		if err := specs.LoadRepos(ctx); err != nil {
			return fmt.Errorf("LoadRepos: %w", err)
		}

		// Loop through each spec and create a schedule task for it
		for _, row := range specs {
			// cancel running jobs if the event is push
			if row.Schedule.Event == webhook_module.HookEventPush {
				// cancel running jobs of the same workflow
				if err := CancelPreviousJobs(
					ctx,
					row.RepoID,
					row.Schedule.Ref,
					row.Schedule.WorkflowID,
					webhook_module.HookEventSchedule,
				); err != nil {
					log.Error("CancelPreviousJobs: %v", err)
				}
			}

			if row.Repo.IsArchived {
				// Skip if the repo is archived
				continue
			}

			cfg, err := row.Repo.GetUnit(ctx, unit.TypeActions)
			if err != nil {
				if repo_model.IsErrUnitTypeNotExist(err) {
					// Skip the actions unit of this repo is disabled.
					continue
				}
				return fmt.Errorf("GetUnit: %w", err)
			}
			if cfg.ActionsConfig().IsWorkflowDisabled(row.Schedule.WorkflowID) {
				continue
			}

			if err := CreateScheduleTask(ctx, row.Schedule); err != nil {
				log.Error("CreateScheduleTask: %v", err)
				return err
			}

			// Parse the spec
			schedule, err := row.Parse()
			if err != nil {
				log.Error("Parse: %v", err)
				return err
			}

			// Update the spec's next run time and previous run time
			row.Prev = row.Next
			row.Next = timeutil.TimeStamp(schedule.Next(now.Add(1 * time.Minute)).Unix())
			if err := actions_model.UpdateScheduleSpec(ctx, row, "prev", "next"); err != nil {
				log.Error("UpdateScheduleSpec: %v", err)
				return err
			}
		}

		// Stop if all specs have been retrieved
		if len(specs) < pageSize {
			break
		}
	}

	return nil
}

// CreateScheduleTask creates a scheduled task from a cron action schedule.
// It creates an action run based on the schedule, inserts it into the database, and creates commit statuses for each job.
func CreateScheduleTask(ctx context.Context, cron *actions_model.ActionSchedule) error {
	// Create a new action run based on the schedule
	run := &actions_model.ActionRun{
		Title:         cron.Title,
		RepoID:        cron.RepoID,
		OwnerID:       cron.OwnerID,
		WorkflowID:    cron.WorkflowID,
		TriggerUserID: cron.TriggerUserID,
		Ref:           cron.Ref,
		CommitSHA:     cron.CommitSHA,
		Event:         cron.Event,
		EventPayload:  cron.EventPayload,
		TriggerEvent:  string(webhook_module.HookEventSchedule),
		ScheduleID:    cron.ID,
		Status:        actions_model.StatusWaiting,
	}

	vars, err := actions_model.GetVariablesOfRun(ctx, run)
	if err != nil {
		log.Error("GetVariablesOfRun: %v", err)
		return err
	}

	// Parse the workflow specification from the cron schedule
	workflows, err := jobparser.Parse(cron.Content, jobparser.WithVars(vars))
	if err != nil {
		return err
	}

	// Insert the action run and its associated jobs into the database
	if err := actions_model.InsertRun(ctx, run, workflows); err != nil {
		return err
	}
	allJobs, err := db.Find[actions_model.ActionRunJob](ctx, actions_model.FindRunJobOptions{RunID: run.ID})
	if err != nil {
		log.Error("FindRunJobs: %v", err)
	}
	err = run.LoadAttributes(ctx)
	if err != nil {
		log.Error("LoadAttributes: %v", err)
	}
	for _, job := range allJobs {
		notify_service.WorkflowJobStatusUpdate(ctx, run.Repo, run.TriggerUser, job, nil)
	}

	// Return nil if no errors occurred
	return nil
}
