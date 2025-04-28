// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cron

import (
	"context"
	"time"

	"code.gitea.io/gitea/models"
	git_model "code.gitea.io/gitea/models/git"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/auth"
	"code.gitea.io/gitea/services/migrations"
	mirror_service "code.gitea.io/gitea/services/mirror"
	packages_cleanup_service "code.gitea.io/gitea/services/packages/cleanup"
	repo_service "code.gitea.io/gitea/services/repository"
	archiver_service "code.gitea.io/gitea/services/repository/archiver"
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
	}, func(ctx context.Context, _ *user_model.User, cfg Config) error {
		umtc := cfg.(*UpdateMirrorTaskConfig)
		return mirror_service.Update(ctx, umtc.PullLimit, umtc.PushLimit)
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
		Timeout: time.Duration(setting.Git.Timeout.Default) * time.Second,
		Args:    []string{},
	}, func(ctx context.Context, _ *user_model.User, config Config) error {
		rhcConfig := config.(*RepoHealthCheckConfig)
		// the git args are set by config, they can be safe to be trusted
		return repo_service.GitFsckRepos(ctx, rhcConfig.Timeout, git.ToTrustedCmdArgs(rhcConfig.Args))
	})
}

func registerCheckRepoStats() {
	RegisterTaskFatal("check_repo_stats", &BaseConfig{
		Enabled:    true,
		RunAtStart: true,
		Schedule:   "@midnight",
	}, func(ctx context.Context, _ *user_model.User, _ Config) error {
		return models.CheckRepoStats(ctx)
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
	}, func(ctx context.Context, _ *user_model.User, config Config) error {
		acConfig := config.(*OlderThanConfig)
		return archiver_service.DeleteOldRepositoryArchives(ctx, acConfig.OlderThan)
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
	}, func(ctx context.Context, _ *user_model.User, config Config) error {
		realConfig := config.(*UpdateExistingConfig)
		return auth.SyncExternalUsers(ctx, realConfig.UpdateExisting)
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
	}, func(ctx context.Context, _ *user_model.User, config Config) error {
		realConfig := config.(*OlderThanConfig)
		git_model.RemoveOldDeletedBranches(ctx, realConfig.OlderThan)
		return nil
	})
}

func registerUpdateMigrationPosterID() {
	RegisterTaskFatal("update_migration_poster_id", &BaseConfig{
		Enabled:    true,
		RunAtStart: true,
		Schedule:   "@midnight",
	}, func(ctx context.Context, _ *user_model.User, _ Config) error {
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
	}, func(ctx context.Context, _ *user_model.User, config Config) error {
		realConfig := config.(*CleanupHookTaskConfig)
		return webhook.CleanupHookTaskTable(ctx, webhook.ToHookTaskCleanupType(realConfig.CleanupType), realConfig.OlderThan, realConfig.NumberToKeep)
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
	}, func(ctx context.Context, _ *user_model.User, config Config) error {
		realConfig := config.(*OlderThanConfig)
		return packages_cleanup_service.CleanupTask(ctx, realConfig.OlderThan)
	})
}

func registerSyncRepoLicenses() {
	RegisterTaskFatal("sync_repo_licenses", &BaseConfig{
		Enabled:    false,
		RunAtStart: false,
		Schedule:   "@annually",
	}, func(ctx context.Context, _ *user_model.User, config Config) error {
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
