// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cron

import (
	"time"

	"code.gitea.io/gitea/modules/translation"
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

// FormatMessage returns a message for the task
// Please note the `status` string will be concatenated with `admin.dashboard.cron.` and `admin.dashboard.task.` to provide locale messages. Similarly `name` will be composed with `admin.dashboard.` to provide the locale name for the task.
func (b *BaseConfig) FormatMessage(locale translation.Locale, name, status, doer string, args ...any) string {
	realArgs := make([]any, 0, len(args)+2)
	realArgs = append(realArgs, locale.Tr("admin.dashboard."+name))
	if doer == "" {
		realArgs = append(realArgs, "(Cron)")
	} else {
		realArgs = append(realArgs, doer)
	}
	if len(args) > 0 {
		realArgs = append(realArgs, args...)
	}
	if doer == "" {
		return locale.Tr("admin.dashboard.cron."+status, realArgs...)
	}
	return locale.Tr("admin.dashboard.task."+status, realArgs...)
}
