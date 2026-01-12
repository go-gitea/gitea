// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestActionRunner_Capacity(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	ctx := context.Background()

	t.Run("DefaultCapacity", func(t *testing.T) {
		runner := &ActionRunner{
			UUID:      "test-uuid-1",
			Name:      "test-runner",
			OwnerID:   0,
			RepoID:    0,
			TokenHash: "hash1",
			Token:     "token1",
		}
		assert.NoError(t, db.Insert(ctx, runner))

		// Verify in database
		retrieved, err := GetRunnerByID(ctx, runner.ID)
		assert.NoError(t, err)
		assert.Equal(t, 0, retrieved.Capacity, "Default capacity should be 0 (unlimited)")
	})

	t.Run("CustomCapacity", func(t *testing.T) {
		runner := &ActionRunner{
			UUID:      "test-uuid-2",
			Name:      "test-runner-2",
			OwnerID:   0,
			RepoID:    0,
			Capacity:  5,
			TokenHash: "hash2",
			Token:     "token2",
		}
		assert.NoError(t, db.Insert(ctx, runner))

		assert.Equal(t, 5, runner.Capacity)

		// Verify in database
		retrieved, err := GetRunnerByID(ctx, runner.ID)
		assert.NoError(t, err)
		assert.Equal(t, 5, retrieved.Capacity)
	})

	t.Run("UpdateCapacity", func(t *testing.T) {
		runner := &ActionRunner{
			UUID:      "test-uuid-3",
			Name:      "test-runner-3",
			OwnerID:   0,
			RepoID:    0,
			Capacity:  1,
			TokenHash: "hash3",
			Token:     "token3",
		}
		assert.NoError(t, db.Insert(ctx, runner))

		// Update capacity
		runner.Capacity = 10
		assert.NoError(t, UpdateRunner(ctx, runner, "capacity"))

		// Verify update
		retrieved, err := GetRunnerByID(ctx, runner.ID)
		assert.NoError(t, err)
		assert.Equal(t, 10, retrieved.Capacity)
	})

	t.Run("ZeroCapacity", func(t *testing.T) {
		runner := &ActionRunner{
			UUID:     "test-uuid-4",
			Name:     "test-runner-4",
			OwnerID:  0,
			RepoID:   0,
			Capacity: 0, // Unlimited
		}
		assert.NoError(t, db.Insert(ctx, runner))

		assert.Equal(t, 0, runner.Capacity)

		retrieved, err := GetRunnerByID(ctx, runner.ID)
		assert.NoError(t, err)
		assert.Equal(t, 0, retrieved.Capacity)
	})
}
