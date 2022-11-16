// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"context"
	"time"

	bots_model "code.gitea.io/gitea/models/bots"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"

	runnerv1 "gitea.com/gitea/proto-go/runner/v1"
)

const (
	zombieTaskTimeout   = 10 * time.Minute
	endlessTaskTimeout  = 3 * time.Hour
	abandonedJobTimeout = 24 * time.Hour
)

// StopZombieTasks stops the task which have running status, but haven't been updated for a long time
func StopZombieTasks(ctx context.Context) error {
	tasks, _, err := bots_model.FindTasks(ctx, bots_model.FindTaskOptions{
		Status:        bots_model.StatusRunning,
		UpdatedBefore: timeutil.TimeStamp(time.Now().Add(-zombieTaskTimeout).Unix()),
	})
	if err != nil {
		log.Warn("find zombie tasks: %v", err)
		return err
	}

	for _, task := range tasks {
		if _, err := bots_model.StopTask(ctx, task, runnerv1.Result_RESULT_FAILURE); err != nil {
			log.Warn("stop zombie task %v: %v", task.ID, err)
			// go on
		}
	}
	return nil
}

// StopEndlessTasks stops the tasks which have running status and continuous updates, but don't end for a long time
func StopEndlessTasks(ctx context.Context) error {
	tasks, _, err := bots_model.FindTasks(ctx, bots_model.FindTaskOptions{
		Status:        bots_model.StatusRunning,
		StartedBefore: timeutil.TimeStamp(time.Now().Add(-endlessTaskTimeout).Unix()),
	})
	if err != nil {
		log.Warn("find endless tasks: %v", err)
		return err
	}

	for _, task := range tasks {
		if _, err := bots_model.StopTask(ctx, task, runnerv1.Result_RESULT_FAILURE); err != nil {
			log.Warn("stop endless task %v: %v", task.ID, err)
			// go on
		}
	}
	return nil
}

// CancelAbandonedJobs cancels the jobs which have waiting status, but haven't been picked by a runner for a long time
func CancelAbandonedJobs(ctx context.Context) error {
	jobs, _, err := bots_model.FindRunJobs(ctx, bots_model.FindRunJobOptions{
		Statuses:      []bots_model.Status{bots_model.StatusWaiting, bots_model.StatusBlocked},
		UpdatedBefore: timeutil.TimeStamp(time.Now().Add(-abandonedJobTimeout).Unix()),
	})
	if err != nil {
		log.Warn("find abandoned tasks: %v", err)
		return err
	}

	now := timeutil.TimeStampNow()
	for _, job := range jobs {
		job.Status = bots_model.StatusCancelled
		job.Stopped = now
		if err := db.WithTx(func(ctx context.Context) error {
			_, err := bots_model.UpdateRunJob(ctx, job, nil, "status", "stopped")
			return err
		}, ctx); err != nil {
			log.Warn("cancel abandoned job %v: %v", job.ID, err)
			// go on
		}
	}

	return nil
}
