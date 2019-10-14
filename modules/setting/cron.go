// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"time"

	"code.gitea.io/gitea/modules/log"
)

var (

	// Cron tasks
	Cron = struct {
		UpdateMirror struct {
			Enabled    bool
			RunAtStart bool
			Schedule   string
		} `ini:"cron.update_mirrors"`
		RepoHealthCheck struct {
			Enabled    bool
			RunAtStart bool
			Schedule   string
			Timeout    time.Duration
			Args       []string `delim:" "`
		} `ini:"cron.repo_health_check"`
		CheckRepoStats struct {
			Enabled    bool
			RunAtStart bool
			Schedule   string
		} `ini:"cron.check_repo_stats"`
		ArchiveCleanup struct {
			Enabled    bool
			RunAtStart bool
			Schedule   string
			OlderThan  time.Duration
		} `ini:"cron.archive_cleanup"`
		SyncExternalUsers struct {
			Enabled        bool
			RunAtStart     bool
			Schedule       string
			UpdateExisting bool
		} `ini:"cron.sync_external_users"`
		DeletedBranchesCleanup struct {
			Enabled    bool
			RunAtStart bool
			Schedule   string
			OlderThan  time.Duration
		} `ini:"cron.deleted_branches_cleanup"`
		UpdateMigrationPosterID struct {
			Schedule string
		} `ini:"cron.update_migration_poster_id"`
	}{
		UpdateMirror: struct {
			Enabled    bool
			RunAtStart bool
			Schedule   string
		}{
			Enabled:    true,
			RunAtStart: false,
			Schedule:   "@every 10m",
		},
		RepoHealthCheck: struct {
			Enabled    bool
			RunAtStart bool
			Schedule   string
			Timeout    time.Duration
			Args       []string `delim:" "`
		}{
			Enabled:    true,
			RunAtStart: false,
			Schedule:   "@every 24h",
			Timeout:    60 * time.Second,
			Args:       []string{},
		},
		CheckRepoStats: struct {
			Enabled    bool
			RunAtStart bool
			Schedule   string
		}{
			Enabled:    true,
			RunAtStart: true,
			Schedule:   "@every 24h",
		},
		ArchiveCleanup: struct {
			Enabled    bool
			RunAtStart bool
			Schedule   string
			OlderThan  time.Duration
		}{
			Enabled:    true,
			RunAtStart: true,
			Schedule:   "@every 24h",
			OlderThan:  24 * time.Hour,
		},
		SyncExternalUsers: struct {
			Enabled        bool
			RunAtStart     bool
			Schedule       string
			UpdateExisting bool
		}{
			Enabled:        true,
			RunAtStart:     false,
			Schedule:       "@every 24h",
			UpdateExisting: true,
		},
		DeletedBranchesCleanup: struct {
			Enabled    bool
			RunAtStart bool
			Schedule   string
			OlderThan  time.Duration
		}{
			Enabled:    true,
			RunAtStart: true,
			Schedule:   "@every 24h",
			OlderThan:  24 * time.Hour,
		},
		UpdateMigrationPosterID: struct {
			Schedule string
		}{
			Schedule: "@every 24h",
		},
	}
)

func newCron() {
	if err := Cfg.Section("cron").MapTo(&Cron); err != nil {
		log.Fatal("Failed to map Cron settings: %v", err)
	}
}
