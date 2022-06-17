// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cron

import (
	"time"

	"code.gitea.io/gitea/modules/translation/i18n"
)

// Config represents a basic configuration interface that cron task
type Config interface {
	IsEnabled() bool
	DoRunAtStart() bool
	GetSchedule() string
	FormatMessage(locale, name, status, doer string, args ...interface{}) string
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
func (b *BaseConfig) FormatMessage(locale, name, status, doer string, args ...interface{}) string {
	realArgs := make([]interface{}, 0, len(args)+2)
	realArgs = append(realArgs, i18n.Tr(locale, "admin.dashboard."+name))
	if doer == "" {
		realArgs = append(realArgs, "(Cron)")
	} else {
		realArgs = append(realArgs, doer)
	}
	if len(args) > 0 {
		realArgs = append(realArgs, args...)
	}
	if doer == "" {
		return i18n.Tr(locale, "admin.dashboard.cron."+status, realArgs...)
	}
	return i18n.Tr(locale, "admin.dashboard.task."+status, realArgs...)
}
