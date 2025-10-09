// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"
	"time"

	"code.gitea.io/gitea/modules/webhook"

	"github.com/stretchr/testify/assert"
	"xorm.io/builder"
)

func TestFindRunOptions_ToConds_ExcludePullRequests(t *testing.T) {
	// Test when ExcludePullRequests is true
	opts := FindRunOptions{
		ExcludePullRequests: true,
	}
	cond := opts.ToConds()

	// Convert the condition to SQL for assertion
	sql, args, err := builder.ToSQL(cond)
	assert.NoError(t, err)
	// The condition should contain the trigger_event not equal to pull_request
	assert.Contains(t, sql, "`action_run`.trigger_event<>")
	assert.Contains(t, args, webhook.HookEventPullRequest)
}

func TestFindRunOptions_ToConds_CreatedDateRange(t *testing.T) {
	// Test when CreatedAfter and CreatedBefore are set
	startDate := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC)

	opts := FindRunOptions{
		CreatedAfter:  startDate,
		CreatedBefore: endDate,
	}
	cond := opts.ToConds()

	// Convert the condition to SQL for assertion
	sql, args, err := builder.ToSQL(cond)
	assert.NoError(t, err)
	// The condition should contain created >= startDate and created <= endDate
	assert.Contains(t, sql, "`action_run`.created>=")
	assert.Contains(t, sql, "`action_run`.created<=")
	assert.Contains(t, args, startDate)
	assert.Contains(t, args, endDate)
}

func TestFindRunOptions_ToConds_CreatedAfterOnly(t *testing.T) {
	// Test when only CreatedAfter is set
	startDate := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

	opts := FindRunOptions{
		CreatedAfter: startDate,
	}
	cond := opts.ToConds()

	// Convert the condition to SQL for assertion
	sql, args, err := builder.ToSQL(cond)
	assert.NoError(t, err)
	// The condition should contain created >= startDate
	assert.Contains(t, sql, "`action_run`.created>=")
	assert.Contains(t, args, startDate)
	// But should not contain created <= endDate
	assert.NotContains(t, sql, "`action_run`.created<=")
}

func TestFindRunOptions_ToConds_CreatedBeforeOnly(t *testing.T) {
	// Test when only CreatedBefore is set
	endDate := time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC)

	opts := FindRunOptions{
		CreatedBefore: endDate,
	}
	cond := opts.ToConds()

	// Convert the condition to SQL for assertion
	sql, args, err := builder.ToSQL(cond)
	assert.NoError(t, err)
	// The condition should contain created <= endDate
	assert.Contains(t, sql, "`action_run`.created<=")
	assert.Contains(t, args, endDate)
	// But should not contain created >= startDate
	assert.NotContains(t, sql, "`action_run`.created>=")
}
