// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cron

import (
	"context"

	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	actions_service "gitea.dev/services/actions"
)

func initActionsTasks() {
	if !setting.Actions.Enabled {
		return
	}
	registerStopZombieTasks()
	registerStopEndlessTasks()
	registerCancelAbandonedJobs()
	registerReEmitUnresolvedActionRuns()
	registerScheduleTasks()
	registerActionsCleanup()
	registerCleanupActionRuns()
}

func registerStopZombieTasks() {
	RegisterTaskFatal("stop_zombie_tasks", &BaseConfig{
		Enabled:    true,
		RunAtStart: true,
		Schedule:   "@every 5m",
	}, func(ctx context.Context, _ *user_model.User, _ *BaseConfig) error {
		return actions_service.StopZombieTasks(ctx)
	})
}

func registerStopEndlessTasks() {
	RegisterTaskFatal("stop_endless_tasks", &BaseConfig{
		Enabled:    true,
		RunAtStart: true,
		Schedule:   "@every 30m",
	}, func(ctx context.Context, _ *user_model.User, _ *BaseConfig) error {
		return actions_service.StopEndlessTasks(ctx)
	})
}

func registerCancelAbandonedJobs() {
	RegisterTaskFatal("cancel_abandoned_jobs", &BaseConfig{
		Enabled:    true,
		RunAtStart: true,
		Schedule:   "@every 6h",
	}, func(ctx context.Context, _ *user_model.User, _ *BaseConfig) error {
		return actions_service.CancelAbandonedJobs(ctx)
	})
}

// registerReEmitUnresolvedActionRuns registers a task that recovers runs
// stalled by a lost job-completion transition: a job finished, but the
// in-memory emission that would re-evaluate its dependents never ran, so the
// dependents stay waiting/blocked/pending forever. Running at startup heals
// runs stalled by a crash or restart, and the periodic run heals runs whose
// emission is lost while the server is up.
func registerReEmitUnresolvedActionRuns() {
	RegisterTaskFatal("reemit_unresolved_action_runs", &BaseConfig{
		Enabled:    true,
		RunAtStart: true,
		Schedule:   "@every 5m",
	}, func(ctx context.Context, _ *user_model.User, _ *BaseConfig) error {
		return actions_service.ReEmitUnresolvedRuns(ctx)
	})
}

// registerScheduleTasks registers a scheduled task that runs every minute to start any due schedule tasks.
func registerScheduleTasks() {
	// Register the task with a unique name, enabled status, and schedule for every minute.
	RegisterTaskFatal("start_schedule_tasks", &BaseConfig{
		Enabled:    true,
		RunAtStart: false,
		Schedule:   "@every 1m",
	}, func(ctx context.Context, _ *user_model.User, _ *BaseConfig) error {
		// Call the function to start schedule tasks and pass the context.
		return actions_service.StartScheduleTasks(ctx)
	})
}

func registerActionsCleanup() {
	RegisterTaskFatal("cleanup_actions", &BaseConfig{
		Enabled:    true,
		RunAtStart: false,
		Schedule:   "@midnight",
	}, func(ctx context.Context, _ *user_model.User, _ *BaseConfig) error {
		return actions_service.Cleanup(ctx)
	})
}

func registerCleanupActionRuns() {
	RegisterTaskFatal("cleanup_action_runs", &BaseConfig{
		Enabled:    true,
		RunAtStart: false,
		Schedule:   "@midnight",
	}, func(ctx context.Context, _ *user_model.User, _ *BaseConfig) error {
		return actions_service.CleanupOldRuns(ctx)
	})
}
