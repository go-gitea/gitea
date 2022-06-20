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

// buildQueue is a global queue of bot build
var buildQueue queue.Queue

// PushToQueue
func PushToQueue(task *bots_model.Build) {
	buildQueue.Push(task)
}

// Dispatch assign a task to a runner
func Dispatch(task *bots_model.Build) (*bots_model.Runner, error) {
	runner, err := bots_model.GetUsableRunner(bots_model.FindRunnerOptions{
		RepoID: task.RepoID,
	})
	if err != nil {
		return nil, err
	}

	return runner, bots_model.AssignBuildToRunner(task.ID, runner.ID)
}

// Init will start the service to get all unfinished tasks and run them
func Init() error {
	buildQueue = queue.CreateQueue("actions_task", handle, &bots_model.Build{})
	if buildQueue == nil {
		return fmt.Errorf("Unable to create Task Queue")
	}

	go graceful.GetManager().RunWithShutdownFns(buildQueue.Run)

	return nil
}

func handle(data ...queue.Data) []queue.Data {
	var unhandled []queue.Data
	for _, datum := range data {
		build := datum.(*bots_model.Build)
		runner, err := Dispatch(build)
		if err != nil {
			log.Error("Run build failed: %v", err)
			unhandled = append(unhandled, build)
		} else {
			log.Trace("build %v assigned to %s", build.UUID, runner.UUID)
		}
	}
	return unhandled
}
