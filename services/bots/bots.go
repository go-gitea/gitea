// Copyright 2022 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"fmt"

	bots_model "code.gitea.io/gitea/models/bots"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/queue"
	//"code.gitea.io/gitea/modules/json"
)

// taskQueue is a global queue of tasks
var taskQueue queue.Queue

// PushToQueue
func PushToQueue(task *bots_model.Task) {
	taskQueue.Push(task)
}

// Dispatch assign a task to a runner
func Dispatch(task *bots_model.Task) (*bots_model.Runner, error) {
	runner, err := bots_model.GetUsableRunner(bots_model.GetRunnerOptions{
		RepoID: task.RepoID,
	})
	if err != nil {
		return nil, err
	}

	return runner, bots_model.AssignTaskToRunner(task.ID, runner.ID)
}

// Init will start the service to get all unfinished tasks and run them
func Init() error {
	taskQueue = queue.CreateQueue("actions_task", handle, &bots_model.Task{})
	if taskQueue == nil {
		return fmt.Errorf("Unable to create Task Queue")
	}

	go graceful.GetManager().RunWithShutdownFns(taskQueue.Run)

	return nil
}

func handle(data ...queue.Data) []queue.Data {
	var unhandled []queue.Data
	for _, datum := range data {
		task := datum.(*bots_model.Task)
		runner, err := Dispatch(task)
		if err != nil {
			log.Error("Run task failed: %v", err)
			unhandled = append(unhandled, task)
		} else {
			log.Trace("task %v assigned to %s", task.UUID, runner.UUID)
		}
	}
	return unhandled
}
