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
)

// StartScheduleTasks start the task
func StartScheduleTasks(ctx context.Context) error {
	return startTasks(ctx, actions_model.FindSpecOptions{
		GetAll: true,
	})
}

func startTasks(ctx context.Context, opts actions_model.FindSpecOptions) error {
	specs, count, err := actions_model.FindSpecs(ctx, opts)
	if err != nil {
		return fmt.Errorf("find specs: %w", err)
	}
	if count == 0 {
		return nil
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
