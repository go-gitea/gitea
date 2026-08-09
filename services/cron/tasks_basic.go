// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cron

import (
	"context"
	"time"

	git_model "gitea.dev/models/git"
	"gitea.dev/models/repostats"
	user_model "gitea.dev/models/user"
	"gitea.dev/models/webhook"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/setting"
	"gitea.dev/services/auth"
	"gitea.dev/services/migrations"
	mirror_service "gitea.dev/services/mirror"
	packages_cleanup_service "gitea.dev/services/packages/cleanup"
	repo_service "gitea.dev/services/repository"
	archiver_service "gitea.dev/services/repository/archiver"
)

func registerUpdateMirrorTask() {
	type UpdateMirrorTaskConfig struct {
		BaseConfig
		PullLimit int
		PushLimit int
	}

	RegisterTaskFatal("update_mirrors", &UpdateMirrorTaskConfig{
		BaseConfig: BaseConfig{
			Enabled:    true,
			RunAtStart: false,
			Schedule:   "@every 10m",
		},
		PullLimit: 50,
		PushLimit: 50,
	}, func(ctx context.Context, _ *user_model.User, cfg *UpdateMirrorTaskConfig) error {
		return mirror_service.Update(ctx, cfg.PullLimit, cfg.PushLimit)
	})
}

func registerRepoHealthCheck() {
	type RepoHealthCheckConfig struct {
		BaseConfig
		Timeout time.Duration
		Args    []string `delim:" "`
	}
	RegisterTaskFatal("repo_health_check", &RepoHealthCheckConfig{
		BaseConfig: BaseConfig{
			Enabled:    true,
			RunAtStart: false,
			Schedule:   "@midnight",
		},
		Timeout: time.Duration(setting.Git.Timeout.GC) * time.Second,
		Args:    []string{},
	}, func(ctx context.Context, _ *user_model.User, config *RepoHealthCheckConfig) error {
		// the git args are set by config, they can be safe to be trusted
		return repo_service.GitFsckRepos(ctx, config.Timeout, gitcmd.ToTrustedCmdArgs(config.Args))
	})
}

func registerCheckRepoStats() {
	RegisterTaskFatal("check_repo_stats", &BaseConfig{
		Enabled:    true,
		RunAtStart: true,
		Schedule:   "@midnight",
	}, func(ctx context.Context, _ *user_model.User, _ *BaseConfig) error {
		return repostats.CheckRepoStats(ctx)
	})
}

func registerArchiveCleanup() {
	RegisterTaskFatal("archive_cleanup", &OlderThanConfig{
		BaseConfig: BaseConfig{
			Enabled:    true,
			RunAtStart: true,
			Schedule:   "@midnight",
		},
		OlderThan: 24 * time.Hour,
	}, func(ctx context.Context, _ *user_model.User, config *OlderThanConfig) error {
		return archiver_service.DeleteOldRepositoryArchives(ctx, config.OlderThan)
	})
}

func registerSyncExternalUsers() {
	RegisterTaskFatal("sync_external_users", &UpdateExistingConfig{
		BaseConfig: BaseConfig{
			Enabled:    true,
			RunAtStart: false,
			Schedule:   "@midnight",
		},
		UpdateExisting: true,
	}, func(ctx context.Context, _ *user_model.User, config *UpdateExistingConfig) error {
		return auth.SyncExternalUsers(ctx, config.UpdateExisting)
	})
}

func registerDeletedBranchesCleanup() {
	RegisterTaskFatal("deleted_branches_cleanup", &OlderThanConfig{
		BaseConfig: BaseConfig{
			Enabled:    true,
			RunAtStart: true,
			Schedule:   "@midnight",
		},
		OlderThan: 24 * time.Hour,
	}, func(ctx context.Context, _ *user_model.User, config *OlderThanConfig) error {
		git_model.RemoveOldDeletedBranches(ctx, config.OlderThan)
		return nil
	})
}

func registerUpdateMigrationPosterID() {
	RegisterTaskFatal("update_migration_poster_id", &BaseConfig{
		Enabled:    true,
		RunAtStart: true,
		Schedule:   "@midnight",
	}, func(ctx context.Context, _ *user_model.User, _ *BaseConfig) error {
		return migrations.UpdateMigrationPosterID(ctx)
	})
}

func registerCleanupHookTaskTable() {
	RegisterTaskFatal("cleanup_hook_task_table", &CleanupHookTaskConfig{
		BaseConfig: BaseConfig{
			Enabled:    true,
			RunAtStart: false,
			Schedule:   "@midnight",
		},
		CleanupType:  "OlderThan",
		OlderThan:    168 * time.Hour,
		NumberToKeep: 10,
	}, func(ctx context.Context, _ *user_model.User, config *CleanupHookTaskConfig) error {
		return webhook.CleanupHookTaskTable(ctx, webhook.ToHookTaskCleanupType(config.CleanupType), config.OlderThan, config.NumberToKeep)
	})
}

func registerCleanupPackages() {
	RegisterTaskFatal("cleanup_packages", &OlderThanConfig{
		BaseConfig: BaseConfig{
			Enabled:    true,
			RunAtStart: true,
			Schedule:   "@midnight",
		},
		OlderThan: 24 * time.Hour,
	}, func(ctx context.Context, _ *user_model.User, config *OlderThanConfig) error {
		return packages_cleanup_service.CleanupTask(ctx, config.OlderThan)
	})
}

func registerSyncRepoLicenses() {
	RegisterTaskFatal("sync_repo_licenses", &BaseConfig{
		Enabled:    false,
		RunAtStart: false,
		Schedule:   "@annually",
	}, func(ctx context.Context, _ *user_model.User, _ *BaseConfig) error {
		return repo_service.SyncRepoLicenses(ctx)
	})
}

func initBasicTasks() {
	if setting.Mirror.Enabled {
		registerUpdateMirrorTask()
	}
	registerRepoHealthCheck()
	registerCheckRepoStats()
	registerArchiveCleanup()
	registerSyncExternalUsers()
	registerDeletedBranchesCleanup()
	if !setting.Repository.DisableMigrations {
		registerUpdateMigrationPosterID()
	}
	registerCleanupHookTaskTable()
	if setting.Packages.Enabled {
		registerCleanupPackages()
	}
	registerSyncRepoLicenses()
}
