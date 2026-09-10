// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cron

import (
	"time"

	"gitea.dev/modules/translation"
)

// Config represents a basic configuration interface that cron task
type Config interface {
	IsEnabled() bool
	DoRunAtStart() bool
	GetSchedule() string
	FormatMessage(locale translation.Locale, name, status, doer string, args ...any) string
	DoNoticeOnSuccess() bool
}

// BaseConfig represents the basic config for a Cron task
type BaseConfig struct {
	Enabled         bool
	RunAtStart      bool
	Schedule        string
	NoticeOnSuccess bool
}

// OlderThanConfig represents a cron task with OlderThan setting
type OlderThanConfig struct {
	BaseConfig
	OlderThan time.Duration
}

// UpdateExistingConfig represents a cron task with UpdateExisting setting
type UpdateExistingConfig struct {
	BaseConfig
	UpdateExisting bool
}

// CleanupHookTaskConfig represents a cron task with settings to cleanup hook_task
type CleanupHookTaskConfig struct {
	BaseConfig
	CleanupType  string
	OlderThan    time.Duration
	NumberToKeep int
}

// GetSchedule returns the schedule for the base config
func (b *BaseConfig) GetSchedule() string {
	return b.Schedule
}

// IsEnabled returns the enabled status for the config
func (b *BaseConfig) IsEnabled() bool {
	return b.Enabled
}

// DoRunAtStart returns whether the task should be run at the start
func (b *BaseConfig) DoRunAtStart() bool {
	return b.RunAtStart
}

// DoNoticeOnSuccess returns whether a success notice should be posted
func (b *BaseConfig) DoNoticeOnSuccess() bool {
	return b.NoticeOnSuccess
}

// LocaleTaskName returns the translated dashboard name for a cron task.
func LocaleTaskName(locale translation.Locale, name string) string {
	switch name {
	case "archive_cleanup":
		return locale.TrString("admin.dashboard.archive_cleanup")
	case "cancel_abandoned_jobs":
		return locale.TrString("admin.dashboard.cancel_abandoned_jobs")
	case "check_repo_stats":
		return locale.TrString("admin.dashboard.check_repo_stats")
	case "cleanup_action_runs":
		return locale.TrString("admin.dashboard.cleanup_action_runs")
	case "cleanup_actions":
		return locale.TrString("admin.dashboard.cleanup_actions")
	case "cleanup_hook_task_table":
		return locale.TrString("admin.dashboard.cleanup_hook_task_table")
	case "cleanup_packages":
		return locale.TrString("admin.dashboard.cleanup_packages")
	case "delete_generated_repository_avatars":
		return locale.TrString("admin.dashboard.delete_generated_repository_avatars")
	case "delete_inactive_accounts":
		return locale.TrString("admin.dashboard.delete_inactive_accounts")
	case "delete_missing_repos":
		return locale.TrString("admin.dashboard.delete_missing_repos")
	case "delete_old_actions":
		return locale.TrString("admin.dashboard.delete_old_actions")
	case "delete_old_system_notices":
		return locale.TrString("admin.dashboard.delete_old_system_notices")
	case "delete_repo_archives":
		return locale.TrString("admin.dashboard.delete_repo_archives")
	case "deleted_branches_cleanup":
		return locale.TrString("admin.dashboard.deleted_branches_cleanup")
	case "gc_lfs":
		return locale.TrString("admin.dashboard.gc_lfs")
	case "git_gc_repos":
		return locale.TrString("admin.dashboard.git_gc_repos")
	case "rebuild_issue_indexer":
		return locale.TrString("admin.dashboard.rebuild_issue_indexer")
	case "reinit_missing_repos":
		return locale.TrString("admin.dashboard.reinit_missing_repos")
	case "repo_health_check":
		return locale.TrString("admin.dashboard.repo_health_check")
	case "resync_all_hooks":
		return locale.TrString("admin.dashboard.resync_all_hooks")
	case "resync_all_sshkeys":
		return locale.TrString("admin.dashboard.resync_all_sshkeys")
	case "resync_all_sshprincipals":
		return locale.TrString("admin.dashboard.resync_all_sshprincipals")
	case "start_schedule_tasks":
		return locale.TrString("admin.dashboard.start_schedule_tasks")
	case "stop_endless_tasks":
		return locale.TrString("admin.dashboard.stop_endless_tasks")
	case "stop_zombie_tasks":
		return locale.TrString("admin.dashboard.stop_zombie_tasks")
	case "sync_external_users":
		return locale.TrString("admin.dashboard.sync_external_users")
	case "sync_repo_licenses":
		return locale.TrString("admin.dashboard.sync_repo_licenses")
	case "update_checker":
		return locale.TrString("admin.dashboard.update_checker")
	case "update_migration_poster_id":
		return locale.TrString("admin.dashboard.update_migration_poster_id")
	case "update_mirrors":
		return locale.TrString("admin.dashboard.update_mirrors")
	default:
		return name
	}
}

func localeTaskStatus(locale translation.Locale, status string, isCron bool, args ...any) string {
	if isCron {
		switch status {
		case "started":
			return locale.TrString("admin.dashboard.cron.started", args...)
		case "process":
			return locale.TrString("admin.dashboard.cron.process", args...)
		case "cancelled":
			return locale.TrString("admin.dashboard.cron.cancelled", args...)
		case "error":
			return locale.TrString("admin.dashboard.cron.error", args...)
		case "finished":
			return locale.TrString("admin.dashboard.cron.finished", args...)
		}
	} else {
		switch status {
		case "started":
			return locale.TrString("admin.dashboard.task.started", args...)
		case "process":
			return locale.TrString("admin.dashboard.task.process", args...)
		case "cancelled":
			return locale.TrString("admin.dashboard.task.cancelled", args...)
		case "error":
			return locale.TrString("admin.dashboard.task.error", args...)
		case "finished":
			return locale.TrString("admin.dashboard.task.finished", args...)
		}
	}
	if isCron {
		return locale.TrString("admin.dashboard.cron.process", args...)
	}
	return locale.TrString("admin.dashboard.task.process", args...)
}

// FormatMessage returns a message for the task
func (b *BaseConfig) FormatMessage(locale translation.Locale, name, status, doer string, args ...any) string {
	realArgs := make([]any, 0, len(args)+2)
	realArgs = append(realArgs, LocaleTaskName(locale, name))
	if doer == "" {
		realArgs = append(realArgs, "(Cron)")
	} else {
		realArgs = append(realArgs, doer)
	}
	if len(args) > 0 {
		realArgs = append(realArgs, args...)
	}
	return localeTaskStatus(locale, status, doer == "", realArgs...)
}
