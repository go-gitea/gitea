// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/modules/log"

	"github.com/gogs/cron"
	"github.com/nektos/act/pkg/jobparser"
)

// StartScheduleTasks start the task
func StartScheduleTasks(ctx context.Context) error {
	return startTasks(ctx, actions_model.FindSpecOptions{
		GetAll: true,
	})
}

func startTasks(ctx context.Context, opts actions_model.FindSpecOptions) error {
	specs, _, err := actions_model.FindSpecs(ctx, opts)
	if err != nil {
		return fmt.Errorf("find specs: %w", err)
	}

	now := time.Now()
	for _, row := range specs {
		schedule, err := cron.Parse(row.Spec)
		if err != nil {
			log.Error("ParseSpec: %v", err)
			continue
		}

		next := schedule.Next(now)
		if next.Sub(now) <= 60 {
			if err := CreateScheduleTask(ctx, row.Schedule, row.Spec); err != nil {
				log.Error("CreateScheduleTask: %v", err)
			}
		}

	}

	return nil
}

// CreateScheduleTask creates a scheduled task from a cron action schedule and a spec string.
// It creates an action run based on the schedule, inserts it into the database, and creates commit statuses for each job.
func CreateScheduleTask(ctx context.Context, cron *actions_model.ActionSchedule, spec string) error {
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
		Status:        actions_model.StatusWaiting,
	}

	// Parse the workflow specification from the cron schedule
	workflows, err := jobparser.Parse(cron.Content)
	if err != nil {
		return err
	}

	// Insert the action run and its associated jobs into the database
	if err := actions_model.InsertRun(ctx, run, workflows); err != nil {
		return err
	}

	// Retrieve the jobs for the newly created action run
	jobs, _, err := actions_model.FindRunJobs(ctx, actions_model.FindRunJobOptions{RunID: run.ID})
	if err != nil {
		return err
	}

	// Create commit statuses for each job
	for _, job := range jobs {
		if err := CreateCommitStatus(ctx, job); err != nil {
			return err
		}
	}

	// Return nil if no errors occurred
	return nil
}
