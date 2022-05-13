// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cron

import (
	"context"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/admin"
	asymkey_model "code.gitea.io/gitea/models/asymkey"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/updatechecker"
	repo_service "code.gitea.io/gitea/services/repository"
	archiver_service "code.gitea.io/gitea/services/repository/archiver"
	user_service "code.gitea.io/gitea/services/user"
)

func registerDeleteInactiveUsers() {
	RegisterTaskFatal("delete_inactive_accounts", &OlderThanConfig{
		BaseConfig: BaseConfig{
			Enabled:    false,
			RunAtStart: false,
			Schedule:   "@annually",
		},
		OlderThan: time.Minute * time.Duration(setting.Service.ActiveCodeLives),
	}, func(ctx context.Context, _ *user_model.User, config Config) error {
		olderThanConfig := config.(*OlderThanConfig)
		return user_service.DeleteInactiveUsers(ctx, olderThanConfig.OlderThan)
	})
}

func registerDeleteRepositoryArchives() {
	RegisterTaskFatal("delete_repo_archives", &BaseConfig{
		Enabled:    false,
		RunAtStart: false,
		Schedule:   "@annually",
	}, func(ctx context.Context, _ *user_model.User, _ Config) error {
		return archiver_service.DeleteRepositoryArchives(ctx)
	})
}

func registerGarbageCollectRepositories() {
	type RepoHealthCheckConfig struct {
		BaseConfig
		Timeout time.Duration
		Args    []string `delim:" "`
	}
	RegisterTaskFatal("git_gc_repos", &RepoHealthCheckConfig{
		BaseConfig: BaseConfig{
			Enabled:    false,
			RunAtStart: false,
			Schedule:   "@every 72h",
		},
		Timeout: time.Duration(setting.Git.Timeout.GC) * time.Second,
		Args:    setting.Git.GCArgs,
	}, func(ctx context.Context, _ *user_model.User, config Config) error {
		rhcConfig := config.(*RepoHealthCheckConfig)
		return repo_service.GitGcRepos(ctx, rhcConfig.Timeout, rhcConfig.Args...)
	})
}

func registerRewriteAllPublicKeys() {
	RegisterTaskFatal("resync_all_sshkeys", &BaseConfig{
		Enabled:    false,
		RunAtStart: false,
		Schedule:   "@every 72h",
	}, func(_ context.Context, _ *user_model.User, _ Config) error {
		return asymkey_model.RewriteAllPublicKeys()
	})
}

func registerRewriteAllPrincipalKeys() {
	RegisterTaskFatal("resync_all_sshprincipals", &BaseConfig{
		Enabled:    false,
		RunAtStart: false,
		Schedule:   "@every 72h",
	}, func(_ context.Context, _ *user_model.User, _ Config) error {
		return asymkey_model.RewriteAllPrincipalKeys()
	})
}

func registerRepositoryUpdateHook() {
	RegisterTaskFatal("resync_all_hooks", &BaseConfig{
		Enabled:    false,
		RunAtStart: false,
		Schedule:   "@every 72h",
	}, func(ctx context.Context, _ *user_model.User, _ Config) error {
		return repo_service.SyncRepositoryHooks(ctx)
	})
}

func registerReinitMissingRepositories() {
	RegisterTaskFatal("reinit_missing_repos", &BaseConfig{
		Enabled:    false,
		RunAtStart: false,
		Schedule:   "@every 72h",
	}, func(ctx context.Context, _ *user_model.User, _ Config) error {
		return repo_service.ReinitMissingRepositories(ctx)
	})
}

func registerDeleteMissingRepositories() {
	RegisterTaskFatal("delete_missing_repos", &BaseConfig{
		Enabled:    false,
		RunAtStart: false,
		Schedule:   "@every 72h",
	}, func(ctx context.Context, user *user_model.User, _ Config) error {
		return repo_service.DeleteMissingRepositories(ctx, user)
	})
}

func registerRemoveRandomAvatars() {
	RegisterTaskFatal("delete_generated_repository_avatars", &BaseConfig{
		Enabled:    false,
		RunAtStart: false,
		Schedule:   "@every 72h",
	}, func(ctx context.Context, _ *user_model.User, _ Config) error {
		return repo_service.RemoveRandomAvatars(ctx)
	})
}

func registerDeleteOldActions() {
	RegisterTaskFatal("delete_old_actions", &OlderThanConfig{
		BaseConfig: BaseConfig{
			Enabled:    false,
			RunAtStart: false,
			Schedule:   "@every 168h",
		},
		OlderThan: 365 * 24 * time.Hour,
	}, func(ctx context.Context, _ *user_model.User, config Config) error {
		olderThanConfig := config.(*OlderThanConfig)
		return models.DeleteOldActions(olderThanConfig.OlderThan)
	})
}

func registerUpdateGiteaChecker() {
	type UpdateCheckerConfig struct {
		BaseConfig
		HTTPEndpoint string
	}
	RegisterTaskFatal("update_checker", &UpdateCheckerConfig{
		BaseConfig: BaseConfig{
			Enabled:    true,
			RunAtStart: false,
			Schedule:   "@every 168h",
		},
		HTTPEndpoint: "https://dl.gitea.io/gitea/version.json",
	}, func(ctx context.Context, _ *user_model.User, config Config) error {
		updateCheckerConfig := config.(*UpdateCheckerConfig)
		return updatechecker.GiteaUpdateChecker(updateCheckerConfig.HTTPEndpoint)
	})
}

func registerDeleteOldSystemNotices() {
	RegisterTaskFatal("delete_old_system_notices", &OlderThanConfig{
		BaseConfig: BaseConfig{
			Enabled:    false,
			RunAtStart: false,
			Schedule:   "@every 168h",
		},
		OlderThan: 365 * 24 * time.Hour,
	}, func(ctx context.Context, _ *user_model.User, config Config) error {
		olderThanConfig := config.(*OlderThanConfig)
		return admin.DeleteOldSystemNotices(olderThanConfig.OlderThan)
	})
}

func initExtendedTasks() {
	registerDeleteInactiveUsers()
	registerDeleteRepositoryArchives()
	registerGarbageCollectRepositories()
	registerRewriteAllPublicKeys()
	registerRewriteAllPrincipalKeys()
	registerRepositoryUpdateHook()
	registerReinitMissingRepositories()
	registerDeleteMissingRepositories()
	registerRemoveRandomAvatars()
	registerDeleteOldActions()
	registerUpdateGiteaChecker()
	registerDeleteOldSystemNotices()
}
