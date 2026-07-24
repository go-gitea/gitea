// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"fmt"
	"testing"
	"time"

	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShouldPersistLastOnline(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		last timeutil.TimeStamp
		want bool
	}{
		{
			name: "fresh, skip write",
			last: timeutil.TimeStamp(now.Add(-5 * time.Second).Unix()),
			want: false,
		},
		{
			name: "exactly at interval, write",
			last: timeutil.TimeStamp(now.Add(-RunnerHeartbeatInterval).Unix()),
			want: true,
		},
		{
			name: "stale, write",
			last: timeutil.TimeStamp(now.Add(-2 * RunnerHeartbeatInterval).Unix()),
			want: true,
		},
		{
			name: "zero (never seen), write",
			last: 0,
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ShouldPersistLastOnline(tt.last, now))
		})
	}
}

func TestShouldPersistLastActive(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		last timeutil.TimeStamp
		want bool
	}{
		{
			name: "fresh, skip write",
			last: timeutil.TimeStamp(now.Add(-1 * time.Second).Unix()),
			want: false,
		},
		{
			name: "exactly at interval, write",
			last: timeutil.TimeStamp(now.Add(-RunnerActiveInterval).Unix()),
			want: true,
		},
		{
			name: "stale, write",
			last: timeutil.TimeStamp(now.Add(-2 * RunnerActiveInterval).Unix()),
			want: true,
		},
		{
			name: "zero (never seen), write",
			last: 0,
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ShouldPersistLastActive(tt.last, now))
		})
	}
}

func TestFindRunnerOptions_ToOrders_StableTiebreaker(t *testing.T) {
	// Sorts on a non-unique column must end with the unique id tiebreaker so
	// pagination is deterministic; without it, runners sharing the same
	// last_online or name can appear on more than one page. Sorts already on
	// the unique id need no tiebreaker.
	expected := map[string]string{
		"alphabetically":        "name ASC, id ASC",
		"reversealphabetically": "name DESC, id ASC",
		"newest":                "id DESC",
		"oldest":                "id ASC",
	}
	for sort, want := range expected {
		assert.Equal(t, want, FindRunnerOptions{Sort: sort}.ToOrders(), "sort %q", sort)
	}

	// Status-based sorts rank by the computed status (active/idle/offline) first,
	// then push disabled runners to the bottom of their group (is_disabled ASC),
	// then break ties on last_online and finally the unique id so pagination stays
	// deterministic. The status thresholds are time-dependent, so match by shape.
	assert.Regexp(t, `^CASE WHEN last_online <= \d+ THEN 2 WHEN last_active <= \d+ THEN 1 ELSE 0 END ASC, is_disabled ASC, last_online DESC, id ASC$`, FindRunnerOptions{Sort: "online"}.ToOrders())
	assert.Regexp(t, `^CASE WHEN last_online <= \d+ THEN 2 WHEN last_active <= \d+ THEN 1 ELSE 0 END DESC, is_disabled ASC, last_online ASC, id ASC$`, FindRunnerOptions{Sort: "offline"}.ToOrders())
	assert.Regexp(t, `^CASE WHEN last_online <= \d+ THEN 2 WHEN last_active <= \d+ THEN 1 ELSE 0 END ASC, is_disabled ASC, last_online DESC, id ASC$`, FindRunnerOptions{Sort: ""}.ToOrders())
}

func TestFindRunners_PaginationNoDuplicates(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	// Create several runners that all share the same last_online value so the
	// primary sort key (last_online) is tied for all of them.
	const ownerID = 1000
	const count = 6
	for i := range count {
		runner := &ActionRunner{
			Name:       "paginated-runner",
			UUID:       fmt.Sprintf("PAGINATE-TEST-0000-0000-00000000000%d", i),
			TokenHash:  fmt.Sprintf("paginate-test-token-hash-%d", i),
			OwnerID:    ownerID,
			RepoID:     0,
			LastOnline: 42,
		}
		require.NoError(t, db.Insert(ctx, runner))
	}

	// Page through the runners and ensure every id is returned exactly once.
	seen := make(map[int64]int)
	const pageSize = 2
	for page := 1; ; page++ {
		runners, err := db.Find[ActionRunner](ctx, FindRunnerOptions{
			ListOptions: db.ListOptions{Page: page, PageSize: pageSize},
			OwnerID:     ownerID,
		})
		require.NoError(t, err)
		if len(runners) == 0 {
			break
		}
		for _, r := range runners {
			seen[r.ID]++
		}
	}

	assert.Len(t, seen, count, "each runner should be returned exactly once across all pages")
	for id, n := range seen {
		assert.Equal(t, 1, n, "runner %d appeared on %d pages", id, n)
	}
}

// TestFindRunners_SortByStatusGroupsActiveAndIdle verifies that sorting by status
// ranks runners by their computed status (active, then idle, then offline) instead
// of interleaving idle runners with active ones by raw last_online, and that a
// disabled runner sinks to the bottom of its status group rather than mixing with
// enabled runners.
func TestFindRunners_SortByStatusGroupsActiveAndIdle(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	const ownerID = 1001
	now := time.Now()
	// An idle runner that went online most recently would sort before an active
	// runner when ordering by last_online alone; the status rank must override that.
	insert := func(name string, lastOnline, lastActive time.Time, disabled bool) {
		require.NoError(t, db.Insert(ctx, &ActionRunner{
			Name:       name,
			UUID:       "STATUS-SORT-" + name,
			TokenHash:  "status-sort-token-" + name,
			OwnerID:    ownerID,
			LastOnline: timeutil.TimeStamp(lastOnline.Unix()),
			LastActive: timeutil.TimeStamp(lastActive.Unix()),
			IsDisabled: disabled,
		}))
	}
	// Each disabled runner has the most recent last_online within its status group,
	// so it would sort first in that group if the disabled flag were ignored; the
	// last_active value keeps it in the intended group (offline<idle<active).
	insert("active", now.Add(-8*time.Second), now.Add(-5*time.Second), false)
	insert("active-disabled", now, now.Add(-2*time.Second), true)
	insert("idle", now.Add(-30*time.Second), now.Add(-20*time.Second), false)
	insert("idle-disabled", now, now.Add(-20*time.Second), true)
	insert("offline", now.Add(-3*time.Minute), now.Add(-3*time.Minute), false)
	insert("offline-disabled", now.Add(-90*time.Second), now.Add(-90*time.Second), true)

	names := func(runners []*ActionRunner) []string {
		out := make([]string, len(runners))
		for i, r := range runners {
			out[i] = r.Name
		}
		return out
	}

	// Active group first, then idle, then offline; within each group enabled before disabled.
	runners, err := db.Find[ActionRunner](ctx, FindRunnerOptions{OwnerID: ownerID, Sort: "online"})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"active", "active-disabled",
		"idle", "idle-disabled",
		"offline", "offline-disabled",
	}, names(runners))

	// The descending status sort reverses the group order but still keeps disabled
	// runners at the bottom of their group.
	runners, err = db.Find[ActionRunner](ctx, FindRunnerOptions{OwnerID: ownerID, Sort: "offline"})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"offline", "offline-disabled",
		"idle", "idle-disabled",
		"active", "active-disabled",
	}, names(runners))
}
