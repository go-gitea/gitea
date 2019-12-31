// Copyright 2019 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package task

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/migrations/base"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/structs"
)

// taskQueue is a global queue of tasks
var taskQueue queue.Queue

// Run a task
func Run(t *models.Task) error {
	switch t.Type {
	case structs.TaskTypeMigrateRepo:
		return runMigrateTask(t)
	default:
		return fmt.Errorf("Unknown task type: %d", t.Type)
	}
}

// Init will start the service to get all unfinished tasks and run them
func Init() error {
	taskQueue = queue.CreateQueue("task", handle, &models.Task{})

	if taskQueue == nil {
		return fmt.Errorf("Unable to create Task Queue")
	}

	go graceful.GetManager().RunWithShutdownFns(taskQueue.Run)

	return nil
}

func handle(data ...queue.Data) {
	for _, datum := range data {
		task := datum.(*models.Task)
		if err := Run(task); err != nil {
			log.Error("Run task failed: %v", err)
		}
	}
}

// MigrateRepository add migration repository to task
func MigrateRepository(doer, u *models.User, opts base.MigrateOptions) error {
	task, err := models.CreateMigrateTask(doer, u, opts)
	if err != nil {
		return err
	}

	return taskQueue.Push(task)
}
