// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"context"
	"time"

	bots_model "code.gitea.io/gitea/models/bots"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"
	runnerv1 "gitea.com/gitea/proto-go/runner/v1"
)

const (
	zombieTaskTimeout   = 10 * time.Minute
	endlessTaskTimeout  = 3 * time.Hour  // the task is running for a long time with updates
	abandonedJobTimeout = 24 * time.Hour // the job is waiting for being picked by a runner
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
	// TODO
	return nil
}
