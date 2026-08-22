// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"testing"
	"time"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	"gitea.dev/modules/timeutil"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFindRunnersSortByStatus verifies that sorting by status ranks runners by
// their computed status (active, then idle, then offline) instead of interleaving
// idle runners with active ones by raw last_online, and that a disabled runner
// sinks to the bottom of its status group rather than mixing with enabled runners.
//
// It lives in tests/integration rather than a unit test because the status rank is
// a database-evaluated CASE expression; unit tests only run against SQLite, so the
// expression must be exercised against MySQL/PostgreSQL/MSSQL in CI to catch dialect
// differences.
func TestFindRunnersSortByStatus(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	ctx := t.Context()

	const ownerID = 1001
	now := time.Now()
	// An idle runner that went online most recently would sort before an active
	// runner when ordering by last_online alone; the status rank must override that.
	insert := func(name string, lastOnline, lastActive time.Time, disabled bool) {
		require.NoError(t, db.Insert(ctx, &actions_model.ActionRunner{
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

	names := func(runners []*actions_model.ActionRunner) []string {
		out := make([]string, len(runners))
		for i, r := range runners {
			out[i] = r.Name
		}
		return out
	}

	// Active group first, then idle, then offline; within each group enabled before disabled.
	runners, err := db.Find[actions_model.ActionRunner](ctx, actions_model.FindRunnerOptions{OwnerID: ownerID, Sort: "online"})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"active", "active-disabled",
		"idle", "idle-disabled",
		"offline", "offline-disabled",
	}, names(runners))

	// The descending status sort reverses the group order but still keeps disabled
	// runners at the bottom of their group.
	runners, err = db.Find[actions_model.ActionRunner](ctx, actions_model.FindRunnerOptions{OwnerID: ownerID, Sort: "offline"})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"offline", "offline-disabled",
		"idle", "idle-disabled",
		"active", "active-disabled",
	}, names(runners))
}

// TestFindRunnersPaginationNoDuplicates verifies that the unique id tiebreaker in
// FindRunnerOptions.ToOrders keeps pagination deterministic when the primary sort
// key (last_online) is tied for every runner. It is an integration test so the
// ordering is validated against every supported database, not only SQLite.
func TestFindRunnersPaginationNoDuplicates(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	ctx := t.Context()

	// Create several runners that all share the same last_online value so the
	// primary sort key (last_online) is tied for all of them.
	const ownerID = 1000
	const count = 6
	for i := range count {
		require.NoError(t, db.Insert(ctx, &actions_model.ActionRunner{
			Name:       "paginated-runner",
			UUID:       fmt.Sprintf("PAGINATE-TEST-0000-0000-00000000000%d", i),
			TokenHash:  fmt.Sprintf("paginate-test-token-hash-%d", i),
			OwnerID:    ownerID,
			RepoID:     0,
			LastOnline: 42,
		}))
	}

	// Page through the runners and ensure every id is returned exactly once.
	seen := make(map[int64]int)
	const pageSize = 2
	for page := 1; ; page++ {
		runners, err := db.Find[actions_model.ActionRunner](ctx, actions_model.FindRunnerOptions{
			Page: page, PageSize: pageSize,
			OwnerID: ownerID,
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
