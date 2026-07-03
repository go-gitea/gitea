// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"fmt"
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindRunnerOptions_ToOrders_StableTiebreaker(t *testing.T) {
	// Sorts on a non-unique column must end with the unique id tiebreaker so
	// pagination is deterministic; without it, runners sharing the same
	// last_online or name can appear on more than one page. Sorts already on
	// the unique id need no tiebreaker.
	expected := map[string]string{
		"":                      "last_online DESC, id ASC",
		"online":                "last_online DESC, id ASC",
		"offline":               "last_online ASC, id ASC",
		"alphabetically":        "name ASC, id ASC",
		"reversealphabetically": "name DESC, id ASC",
		"newest":                "id DESC",
		"oldest":                "id ASC",
	}
	for sort, want := range expected {
		assert.Equal(t, want, FindRunnerOptions{Sort: sort}.ToOrders(), "sort %q", sort)
	}
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
