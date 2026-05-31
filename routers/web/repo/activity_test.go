// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestActivityDateRange(t *testing.T) {
	now := time.Date(2026, 5, 31, 12, 0, 0, 0, time.Local)

	period, from, until, custom := activityDateRange("", "", "", now)
	assert.Equal(t, "weekly", period)
	assert.Equal(t, now.Add(-time.Hour*168), from)
	assert.Equal(t, now, until)
	assert.False(t, custom)

	period, from, until, custom = activityDateRange("monthly", "", "", now)
	assert.Equal(t, "monthly", period)
	assert.Equal(t, now.AddDate(0, -1, 0), from)
	assert.Equal(t, now, until)
	assert.False(t, custom)

	period, from, until, custom = activityDateRange("weekly", "2026-04-01", "2026-04-15", now)
	assert.Equal(t, "custom", period)
	assert.Equal(t, time.Date(2026, 4, 1, 0, 0, 0, 0, time.Local), from)
	assert.Equal(t, time.Date(2026, 4, 15, 23, 59, 59, 0, time.Local), until)
	assert.True(t, custom)

	period, from, until, custom = activityDateRange("daily", "2026-04-15", "2026-04-01", now)
	assert.Equal(t, "daily", period)
	assert.Equal(t, now.Add(-time.Hour*24), from)
	assert.Equal(t, now, until)
	assert.False(t, custom)
}
