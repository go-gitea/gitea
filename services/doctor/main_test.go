// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package doctor

import (
	"context"
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/log"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func TestRunChecksSkipsDestructiveFixAfterDatabaseFailure(t *testing.T) {
	runs := map[string]int{}
	checks := []*Check{
		{
			Title:                      "db failure",
			Name:                       "db-failure",
			SkipDatabaseInitialization: true,
			Run: func(ctx context.Context, logger log.Logger, autofix bool) error {
				markDatabaseUntrusted(ctx)
				runs["db-failure"]++
				return assert.AnError
			},
		},
		{
			Title:                      "destructive",
			Name:                       "destructive",
			SkipDatabaseInitialization: true,
			IsDestructive:              true,
			Run: func(ctx context.Context, logger log.Logger, autofix bool) error {
				runs["destructive"]++
				return nil
			},
		},
		{
			Title:                      "non-destructive",
			Name:                       "non-destructive",
			SkipDatabaseInitialization: true,
			Run: func(ctx context.Context, logger log.Logger, autofix bool) error {
				runs["non-destructive"]++
				return nil
			},
		},
	}

	err := RunChecks(t.Context(), false, true, checks)
	assert.NoError(t, err)
	assert.Equal(t, 1, runs["db-failure"])
	assert.Zero(t, runs["destructive"])
	assert.Equal(t, 1, runs["non-destructive"])
}

func TestShouldAnnounceSafeFixMode(t *testing.T) {
	ctx := withRunState(context.Background())
	assert.False(t, shouldAnnounceSafeFixMode(ctx, true))

	markDatabaseUntrusted(ctx)
	assert.True(t, shouldAnnounceSafeFixMode(ctx, true))
	assert.False(t, shouldAnnounceSafeFixMode(ctx, true))
	assert.False(t, shouldAnnounceSafeFixMode(ctx, false))
}
