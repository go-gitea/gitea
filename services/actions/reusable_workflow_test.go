// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"fmt"
	"testing"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckCallerChain_Cycle(t *testing.T) {
	t.Run("DirectCycle", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())
		// A -> A: leaf's CallUses matches its direct parent's.
		chain := buildCallerChain(t,
			"./.gitea/workflows/a.yml",
			"./.gitea/workflows/a.yml",
		)
		err := checkCallerChain(t.Context(), chain[len(chain)-1])
		assert.ErrorContains(t, err, "cycle detected")
	})

	t.Run("IndirectCycle", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())
		// A -> B -> A: leaf's CallUses matches its grandparent's.
		chain := buildCallerChain(t,
			"./.gitea/workflows/a.yml",
			"./.gitea/workflows/b.yml",
			"./.gitea/workflows/a.yml",
		)
		err := checkCallerChain(t.Context(), chain[len(chain)-1])
		assert.ErrorContains(t, err, "cycle detected")
	})

	t.Run("NoCycle", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())
		// Sanity: linear chain with distinct CallUses must not trip cycle detection.
		chain := buildCallerChain(t,
			"./.gitea/workflows/a.yml",
			"./.gitea/workflows/b.yml",
			"./.gitea/workflows/c.yml",
		)
		require.NoError(t, checkCallerChain(t.Context(), chain[len(chain)-1]))
	})
}

func TestCheckCallerChain_DepthLimit(t *testing.T) {
	// top + MaxReusableCallLevels nested callers is the longest accepted; one more exceeds the limit.
	makeDistinctUses := func(n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = fmt.Sprintf("./.gitea/workflows/level%d.yml", i)
		}
		return out
	}

	t.Run("ExactlyAtLimit", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())
		chain := buildCallerChain(t, makeDistinctUses(MaxReusableCallLevels+1)...)
		require.NoError(t, checkCallerChain(t.Context(), chain[len(chain)-1]))
	})

	t.Run("OneOverLimit", func(t *testing.T) {
		require.NoError(t, unittest.PrepareTestDatabase())
		chain := buildCallerChain(t, makeDistinctUses(MaxReusableCallLevels+2)...)
		err := checkCallerChain(t.Context(), chain[len(chain)-1])
		assert.ErrorContains(t, err, "exceeds the maximum nesting level")
	})
}

// buildCallerChain inserts a linear chain of reusable caller jobs in a single run+attempt.
// callerUses[0] is the top-level caller (ParentJobID=0); each subsequent caller is inserted as a child of the previous one.
// Returns the inserted jobs in order (index 0 = top, last = leaf).
func buildCallerChain(t *testing.T, callerUses ...string) []*actions_model.ActionRunJob {
	t.Helper()
	require.NotEmpty(t, callerUses)
	ctx := t.Context()

	run := &actions_model.ActionRun{
		Title:         "caller-chain-test",
		RepoID:        4,
		OwnerID:       1,
		Index:         9601,
		WorkflowID:    "test.yaml",
		TriggerUserID: 1,
		Ref:           "refs/heads/master",
		CommitSHA:     "c2d72f548424103f01ee1dc02889c1e2bff816b0",
		Event:         "push",
		TriggerEvent:  "push",
		EventPayload:  "{}",
		Status:        actions_model.StatusRunning,
	}
	require.NoError(t, db.Insert(ctx, run))

	attempt := &actions_model.ActionRunAttempt{
		RepoID:        run.RepoID,
		RunID:         run.ID,
		Attempt:       1,
		TriggerUserID: 1,
		Status:        actions_model.StatusRunning,
	}
	require.NoError(t, db.Insert(ctx, attempt))

	jobs := make([]*actions_model.ActionRunJob, 0, len(callerUses))
	parentID := int64(0)
	for i, uses := range callerUses {
		job := &actions_model.ActionRunJob{
			RunID:            run.ID,
			RunAttemptID:     attempt.ID,
			RepoID:           run.RepoID,
			OwnerID:          run.OwnerID,
			CommitSHA:        run.CommitSHA,
			Name:             fmt.Sprintf("caller-%d", i),
			JobID:            fmt.Sprintf("caller-%d", i),
			Attempt:          1,
			Status:           actions_model.StatusBlocked,
			AttemptJobID:     int64(i + 1),
			IsReusableCaller: true,
			CallUses:         uses,
			ParentJobID:      parentID,
		}
		require.NoError(t, db.Insert(ctx, job))
		jobs = append(jobs, job)
		parentID = job.ID
	}
	return jobs
}
