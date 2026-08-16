// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"testing"
	"time"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// insertCleanupRun inserts a run and backdates its Created timestamp, because
// xorm would otherwise stamp it with the current time.
func insertCleanupRun(t *testing.T, index int64, status actions_model.Status, created timeutil.TimeStamp) *actions_model.ActionRun {
	t.Helper()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	run := &actions_model.ActionRun{
		Title: "cleanup-run", RepoID: repo.ID, OwnerID: repo.OwnerID, WorkflowID: "test.yaml", Index: index,
		TriggerUserID: 1, Ref: "refs/heads/main",
		CommitSHA: "c2d72f548424103f01ee1dc02889c1e2bff816b0", Event: "push", TriggerEvent: "push",
		Status: status,
	}
	require.NoError(t, db.Insert(t.Context(), run))
	// xorm's "created" tag ignores explicit updates to the column, so backdate it
	// with raw SQL (NoAutoTime is not enough).
	_, err := db.GetEngine(t.Context()).Exec("UPDATE action_run SET created = ? WHERE id = ?", created, run.ID)
	require.NoError(t, err)
	return run
}

func TestCleanupOldRuns(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	oldRetention := setting.Actions.RunRetentionDays
	t.Cleanup(func() { setting.Actions.RunRetentionDays = oldRetention })

	now := timeutil.TimeStampNow()
	old := now.AddDuration(-40 * 24 * time.Hour)

	t.Run("disabled retention is a no-op", func(t *testing.T) {
		setting.Actions.RunRetentionDays = 0
		run := insertCleanupRun(t, 2001, actions_model.StatusSuccess, old)
		require.NoError(t, CleanupOldRuns(t.Context()))
		unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run.ID})
	})

	t.Run("deletes old done runs, keeps recent and in-progress runs", func(t *testing.T) {
		setting.Actions.RunRetentionDays = 30
		oldSuccess := insertCleanupRun(t, 2002, actions_model.StatusSuccess, old)
		oldFailure := insertCleanupRun(t, 2003, actions_model.StatusFailure, old)
		recent := insertCleanupRun(t, 2004, actions_model.StatusSuccess, now.AddDuration(-24*time.Hour))
		oldRunning := insertCleanupRun(t, 2005, actions_model.StatusRunning, old)

		require.NoError(t, CleanupOldRuns(t.Context()))

		unittest.AssertNotExistsBean(t, &actions_model.ActionRun{ID: oldSuccess.ID})
		unittest.AssertNotExistsBean(t, &actions_model.ActionRun{ID: oldFailure.ID})
		unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: recent.ID})
		unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: oldRunning.ID})
	})

	t.Run("deletes more than one batch", func(t *testing.T) {
		setting.Actions.RunRetentionDays = 30
		// cleanupOldRunsBatchSize is 50; insert more than one batch of old runs.
		oldRuns := make([]*actions_model.ActionRun, 0, cleanupOldRunsBatchSize+5)
		for i := range cleanupOldRunsBatchSize + 5 {
			oldRuns = append(oldRuns, insertCleanupRun(t, 2100+int64(i), actions_model.StatusSuccess, old))
		}
		require.NoError(t, CleanupOldRuns(t.Context()))
		for _, run := range oldRuns {
			unittest.AssertNotExistsBean(t, &actions_model.ActionRun{ID: run.ID})
		}
	})
}

func TestCleanupOldRunsLoop(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// PanicInDevOrTesting panics in tests; make it log instead so the failure
	// path of the loop can be exercised.
	oldProd, oldTesting := setting.IsProd, setting.IsInTesting
	setting.IsProd, setting.IsInTesting = true, false
	t.Cleanup(func() { setting.IsProd, setting.IsInTesting = oldProd, oldTesting })

	now := timeutil.TimeStampNow()
	old := now.AddDuration(-40 * 24 * time.Hour)
	doneStatuses := []actions_model.Status{actions_model.StatusSuccess}

	// resetDB clears the action_run table so the loop only sees the runs the
	// current subtest inserts (fixture runs would otherwise be fetched too).
	resetDB := func(t *testing.T) {
		t.Helper()
		require.NoError(t, unittest.PrepareTestDatabase())
		_, err := db.GetEngine(t.Context()).Exec("DELETE FROM action_run")
		require.NoError(t, err)
	}

	t.Run("a single failing run is skipped and not retried", func(t *testing.T) {
		resetDB(t)
		oldest := insertCleanupRun(t, 3001, actions_model.StatusSuccess, old)
		middle := insertCleanupRun(t, 3002, actions_model.StatusSuccess, old)
		newest := insertCleanupRun(t, 3003, actions_model.StatusSuccess, old)

		var calls []int64
		deleteRun := func(ctx context.Context, run *actions_model.ActionRun) error {
			calls = append(calls, run.ID)
			if run.ID == oldest.ID {
				return errors.New("boom")
			}
			_, err := db.DeleteByID[actions_model.ActionRun](ctx, run.ID)
			return err
		}

		total, err := cleanupOldRuns(t.Context(), now, doneStatuses, deleteRun)
		require.NoError(t, err)
		assert.Equal(t, 2, total)
		// the failing run is attempted once, then skipped in later batches
		assert.Equal(t, []int64{oldest.ID, middle.ID, newest.ID}, calls)
		unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: oldest.ID})
		unittest.AssertNotExistsBean(t, &actions_model.ActionRun{ID: middle.ID})
		unittest.AssertNotExistsBean(t, &actions_model.ActionRun{ID: newest.ID})
	})

	t.Run("all runs failing terminates", func(t *testing.T) {
		resetDB(t)
		run1 := insertCleanupRun(t, 3011, actions_model.StatusSuccess, old)
		run2 := insertCleanupRun(t, 3012, actions_model.StatusSuccess, old)

		var calls []int64
		deleteRun := func(ctx context.Context, run *actions_model.ActionRun) error {
			calls = append(calls, run.ID)
			return errors.New("boom")
		}

		total, err := cleanupOldRuns(t.Context(), now, doneStatuses, deleteRun)
		require.NoError(t, err)
		assert.Equal(t, 0, total)
		// each run is attempted once, then the loop stops instead of retrying forever
		assert.Equal(t, []int64{run1.ID, run2.ID}, calls)
		unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run1.ID})
		unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run2.ID})
	})

	t.Run("deletes oldest first", func(t *testing.T) {
		resetDB(t)
		run1 := insertCleanupRun(t, 3021, actions_model.StatusSuccess, old)
		run2 := insertCleanupRun(t, 3022, actions_model.StatusSuccess, old)
		run3 := insertCleanupRun(t, 3023, actions_model.StatusSuccess, old)

		var order []int64
		deleteRun := func(ctx context.Context, run *actions_model.ActionRun) error {
			order = append(order, run.ID)
			_, err := db.DeleteByID[actions_model.ActionRun](ctx, run.ID)
			return err
		}

		total, err := cleanupOldRuns(t.Context(), now, doneStatuses, deleteRun)
		require.NoError(t, err)
		assert.Equal(t, 3, total)
		assert.Equal(t, []int64{run1.ID, run2.ID, run3.ID}, order)
	})
}
