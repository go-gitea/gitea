// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"
)

const (
	zombieTaskTimeout   = 10 * time.Minute
	endlessTaskTimeout  = 3 * time.Hour
	abandonedJobTimeout = 24 * time.Hour
)

// StopZombieTasks stops the task which have running status, but haven't been updated for a long time
func StopZombieTasks(ctx context.Context) error {
	tasks, _, err := actions_model.FindTasks(ctx, actions_model.FindTaskOptions{
		Status:        actions_model.StatusRunning,
		UpdatedBefore: timeutil.TimeStamp(time.Now().Add(-zombieTaskTimeout).Unix()),
	})
	if err != nil {
		log.Warn("find zombie tasks: %v", err)
		return err
	}

	for _, task := range tasks {
		if err := db.WithTx(ctx, func(ctx context.Context) error {
			if err := actions_model.StopTask(ctx, task.ID, actions_model.StatusFailure); err != nil {
				return err
			}
			if err := task.LoadJob(ctx); err != nil {
				return err
			}
			return CreateCommitStatus(ctx, task.Job)
		}); err != nil {
			log.Warn("stop zombie task %v: %v", task.ID, err)
			// go on
		}
	}
	return nil
}

// StopEndlessTasks stops the tasks which have running status and continuous updates, but don't end for a long time
func StopEndlessTasks(ctx context.Context) error {
	tasks, _, err := actions_model.FindTasks(ctx, actions_model.FindTaskOptions{
		Status:        actions_model.StatusRunning,
		StartedBefore: timeutil.TimeStamp(time.Now().Add(-endlessTaskTimeout).Unix()),
	})
	if err != nil {
		log.Warn("find endless tasks: %v", err)
		return err
	}

	for _, task := range tasks {
		if err := db.WithTx(ctx, func(ctx context.Context) error {
			if err := actions_model.StopTask(ctx, task.ID, actions_model.StatusFailure); err != nil {
				return err
			}
			if err := task.LoadJob(ctx); err != nil {
				return err
			}
			return CreateCommitStatus(ctx, task.Job)
		}); err != nil {
			log.Warn("stop endless task %v: %v", task.ID, err)
			// go on
		}
	}
	return nil
}

// CancelAbandonedJobs cancels the jobs which have waiting status, but haven't been picked by a runner for a long time
func CancelAbandonedJobs(ctx context.Context) error {
	jobs, _, err := actions_model.FindRunJobs(ctx, actions_model.FindRunJobOptions{
		Statuses:      []actions_model.Status{actions_model.StatusWaiting, actions_model.StatusBlocked},
		UpdatedBefore: timeutil.TimeStamp(time.Now().Add(-abandonedJobTimeout).Unix()),
	})
	if err != nil {
		log.Warn("find abandoned tasks: %v", err)
		return err
	}

	now := timeutil.TimeStampNow()
	for _, job := range jobs {
		job.Status = actions_model.StatusCancelled
		job.Stopped = now
		if err := db.WithTx(ctx, func(ctx context.Context) error {
			if _, err := actions_model.UpdateRunJob(ctx, job, nil, "status", "stopped"); err != nil {
				return err
			}
			return CreateCommitStatus(ctx, job)
		}); err != nil {
			log.Warn("cancel abandoned job %v: %v", job.ID, err)
			// go on
		}
	}

	return nil
}
