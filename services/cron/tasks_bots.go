// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cron

import (
	"context"

	user_model "code.gitea.io/gitea/models/user"
	bots_service "code.gitea.io/gitea/services/bots"
)

func initBotsTasks() {
	registerStopZombieTasks()
	registerStopEndlessTasks()
	registerCancelAbandonedJobs()
}

func registerStopZombieTasks() {
	RegisterTaskFatal("stop_zombie_tasks", &BaseConfig{
		Enabled:    true,
		RunAtStart: true,
		Schedule:   "@every 5m",
	}, func(ctx context.Context, _ *user_model.User, cfg Config) error {
		return bots_service.StopZombieTasks(ctx)
	})
}

func registerStopEndlessTasks() {
	RegisterTaskFatal("stop_endless_tasks", &BaseConfig{
		Enabled:    true,
		RunAtStart: true,
		Schedule:   "@every 30m",
	}, func(ctx context.Context, _ *user_model.User, cfg Config) error {
		return bots_service.StopEndlessTasks(ctx)
	})
}

func registerCancelAbandonedJobs() {
	RegisterTaskFatal("cancel_abandoned_jobs", &BaseConfig{
		Enabled:    true,
		RunAtStart: true,
		Schedule:   "@every 6h",
	}, func(ctx context.Context, _ *user_model.User, cfg Config) error {
		return bots_service.CancelAbandonedJobs(ctx)
	})
}
