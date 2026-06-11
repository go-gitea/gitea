// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"fmt"
	"testing"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/actions/jobparser"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

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

func TestResolveUses(t *testing.T) {
	defer test.MockVariableValue(&setting.AppURL, "https://gitea.example.com/sub/")()
	defer test.MockVariableValue(&setting.AppSubURL, "/sub")()
	ctx := t.Context()

	t.Run("LocalForms", func(t *testing.T) {
		// Same-repo and cross-repo forms are not URLs and are parsed as-is.
		ref, err := ResolveUses(ctx, "./.gitea/workflows/build.yml")
		require.NoError(t, err)
		assert.Equal(t, jobparser.UsesRef{Kind: jobparser.UsesKindLocalSameRepo, Path: ".gitea/workflows/build.yml"}, *ref)

		ref, err = ResolveUses(ctx, "owner/repo/.gitea/workflows/build.yml@v1")
		require.NoError(t, err)
		assert.Equal(t, jobparser.UsesRef{Kind: jobparser.UsesKindLocalCrossRepo, Owner: "owner", Repo: "repo", Path: ".gitea/workflows/build.yml", Ref: "v1"}, *ref)
	})

	t.Run("LocalInstanceURL", func(t *testing.T) {
		// An absolute URL on this instance (incl. AppSubURL) resolves to the equivalent cross-repo ref.
		ref, err := ResolveUses(ctx, "https://gitea.example.com/sub/owner/repo/.gitea/workflows/ci.yml@refs/heads/main")
		require.NoError(t, err)
		assert.Equal(t, jobparser.UsesRef{Kind: jobparser.UsesKindLocalCrossRepo, Owner: "owner", Repo: "repo", Path: ".gitea/workflows/ci.yml", Ref: "refs/heads/main"}, *ref)
	})

	t.Run("InvalidSyntax", func(t *testing.T) {
		for _, in := range []string{
			"owner/.gitea/workflows/foo.yml",                                             // missing repo segment
			"owner/repo/.gitea/workflows/foo.yml",                                        // missing @ref
			"https://gitea.example.com/sub/repo/.gitea/workflows/ci.yml@refs/heads/main", // local absolute URL but missing owner
			"not a valid uses at all",
		} {
			_, err := ResolveUses(ctx, in)
			require.Error(t, err, "in = %s", in)
		}
	})

	t.Run("ForeignURL", func(t *testing.T) {
		_, err := ResolveUses(ctx, "https://other.gitea-example.com/owner/repo/.gitea/workflows/ci.yaml@v1")
		assert.ErrorContains(t, err, "must point to this Gitea instance")
	})
}
