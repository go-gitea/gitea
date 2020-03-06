// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cron

import (
	"context"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/migrations"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/sync"
	mirror_service "code.gitea.io/gitea/services/mirror"

	"github.com/gogs/cron"
)

const (
	mirrorUpdate            = "mirror_update"
	gitFsck                 = "git_fsck"
	checkRepos              = "check_repos"
	archiveCleanup          = "archive_cleanup"
	syncExternalUsers       = "sync_external_users"
	deletedBranchesCleanup  = "deleted_branches_cleanup"
	updateMigrationPosterID = "update_migration_post_id"
)

var c = cron.New()

// Prevent duplicate running tasks.
var taskStatusTable = sync.NewStatusTable()

// Func defines a cron function body
type Func func()

// WithUnique wrap a cron func with an unique running check
func WithUnique(name string, body func(context.Context)) Func {
	return func() {
		if !taskStatusTable.StartIfNotRunning(name) {
			return
		}
		defer taskStatusTable.Stop(name)
		graceful.GetManager().RunWithShutdownContext(body)
	}
}

// UpdateMirror runs the UpdateMirror cron task uniquely
func UpdateMirror() {
	WithUnique(mirrorUpdate, mirror_service.Update)()
}

// RepoHealthCheck runs the RepoHealthCheck cron task uniquely
func RepoHealthCheck() {
	WithUnique(gitFsck, func(ctx context.Context) {
		if err := repo_module.GitFsck(ctx); err != nil {
			log.Error("Error whilst performing repository health checks (git fsck): %s", err)
		}
	})()
}

// CheckRepoStats checks the repository statistics uniquely
func CheckRepoStats() {
	WithUnique(checkRepos, models.CheckRepoStats)()
}

// ArchiveCleanup cleans the archives uniquely
func ArchiveCleanup() {
	WithUnique(archiveCleanup, models.DeleteOldRepositoryArchives)()
}

// SyncExternalUsers syncs external users uniquely
func SyncExternalUsers() {
	WithUnique(syncExternalUsers, models.SyncExternalUsers)()
}

// DeletedBranchesCleanup performs branch deletion cleanup uniquely
func DeletedBranchesCleanup() {
	WithUnique(deletedBranchesCleanup, models.RemoveOldDeletedBranches)()
}

// UpdateMigrationPosterID updates the migrations uniquely
func UpdateMigrationPosterID() {
	WithUnique(updateMigrationPosterID, migrations.UpdateMigrationPosterID)()
}

// NewContext begins cron tasks
// Each cron task is run within the shutdown context as a running server
// AtShutdown the cron server is stopped
func NewContext() {
	var (
		entry *cron.Entry
		err   error
	)
	if setting.Cron.UpdateMirror.Enabled {
		entry, err = c.AddFunc("Update mirrors", setting.Cron.UpdateMirror.Schedule, UpdateMirror)
		if err != nil {
			log.Fatal("Cron[Update mirrors]: %v", err)
		}
		if setting.Cron.UpdateMirror.RunAtStart {
			entry.Prev = time.Now()
			entry.ExecTimes++
			go UpdateMirror()
		}
	}
	if setting.Cron.RepoHealthCheck.Enabled {
		entry, err = c.AddFunc("Repository health check", setting.Cron.RepoHealthCheck.Schedule, RepoHealthCheck)
		if err != nil {
			log.Fatal("Cron[Repository health check]: %v", err)
		}
		if setting.Cron.RepoHealthCheck.RunAtStart {
			entry.Prev = time.Now()
			entry.ExecTimes++
			go RepoHealthCheck()
		}
	}
	if setting.Cron.CheckRepoStats.Enabled {
		entry, err = c.AddFunc("Check repository statistics", setting.Cron.CheckRepoStats.Schedule, CheckRepoStats)
		if err != nil {
			log.Fatal("Cron[Check repository statistics]: %v", err)
		}
		if setting.Cron.CheckRepoStats.RunAtStart {
			entry.Prev = time.Now()
			entry.ExecTimes++
			go CheckRepoStats()
		}
	}
	if setting.Cron.ArchiveCleanup.Enabled {
		entry, err = c.AddFunc("Clean up old repository archives", setting.Cron.ArchiveCleanup.Schedule, ArchiveCleanup)
		if err != nil {
			log.Fatal("Cron[Clean up old repository archives]: %v", err)
		}
		if setting.Cron.ArchiveCleanup.RunAtStart {
			entry.Prev = time.Now()
			entry.ExecTimes++
			go ArchiveCleanup()
		}
	}
	if setting.Cron.SyncExternalUsers.Enabled {
		entry, err = c.AddFunc("Synchronize external users", setting.Cron.SyncExternalUsers.Schedule, SyncExternalUsers)
		if err != nil {
			log.Fatal("Cron[Synchronize external users]: %v", err)
		}
		if setting.Cron.SyncExternalUsers.RunAtStart {
			entry.Prev = time.Now()
			entry.ExecTimes++
			go SyncExternalUsers()
		}
	}
	if setting.Cron.DeletedBranchesCleanup.Enabled {
		entry, err = c.AddFunc("Remove old deleted branches", setting.Cron.DeletedBranchesCleanup.Schedule, DeletedBranchesCleanup)
		if err != nil {
			log.Fatal("Cron[Remove old deleted branches]: %v", err)
		}
		if setting.Cron.DeletedBranchesCleanup.RunAtStart {
			entry.Prev = time.Now()
			entry.ExecTimes++
			go DeletedBranchesCleanup()
		}
	}

	entry, err = c.AddFunc("Update migrated repositories' issues and comments' posterid", setting.Cron.UpdateMigrationPosterID.Schedule, UpdateMigrationPosterID)
	if err != nil {
		log.Fatal("Cron[Update migrated repositories]: %v", err)
	}
	entry.Prev = time.Now()
	entry.ExecTimes++
	go UpdateMigrationPosterID()

	c.Start()
	graceful.GetManager().RunAtShutdown(context.Background(), c.Stop)
}

// ListTasks returns all running cron tasks.
func ListTasks() []*cron.Entry {
	return c.Entries()
}
