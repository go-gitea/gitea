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
	"gitea.dev/modules/container"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	// XORM's "created" tag ignores explicit updates to the column even with NoAutoTime, so backdate it with raw SQL.
	_, err := db.GetEngine(t.Context()).Exec("UPDATE action_run SET created = ? WHERE id = ?", created, run.ID)
	require.NoError(t, err)
	return run
}

func deleteAllRuns(t *testing.T) {
	t.Helper()
	_, err := db.GetEngine(t.Context()).Exec("DELETE FROM action_run")
	require.NoError(t, err)
}

func TestCleanupOldRuns(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	defer test.MockVariableValue(&setting.Actions.RunRetentionDays, 30)()

	now := timeutil.TimeStampNow()
	old := now.AddDuration(-40 * 24 * time.Hour)
	deleteAllRuns(t)

	t.Run("disabled retention is a no-op", func(t *testing.T) {
		defer test.MockVariableValue(&setting.Actions.RunRetentionDays, 0)()
		run := insertCleanupRun(t, 2001, actions_model.StatusSuccess, old)
		require.NoError(t, CleanupOldRuns(t.Context()))
		unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run.ID})
	})

	t.Run("deletes old done runs, keeps recent and in-progress runs", func(t *testing.T) {
		defer test.MockVariableValue(&cleanupOldRunsBatchSize, 3)() // also test the batch
		oldSuccess := insertCleanupRun(t, 2002, actions_model.StatusSuccess, old)
		oldFailure := insertCleanupRun(t, 2003, actions_model.StatusFailure, old)
		recent := insertCleanupRun(t, 2004, actions_model.StatusSuccess, now.AddDuration(-24*time.Hour))
		oldRunning := insertCleanupRun(t, 2005, actions_model.StatusRunning, old)
		oldCanceled := insertCleanupRun(t, 2006, actions_model.StatusCancelled, old)
		oldSkipped := insertCleanupRun(t, 2007, actions_model.StatusSkipped, old)

		require.NoError(t, CleanupOldRuns(t.Context()))

		unittest.AssertNotExistsBean(t, &actions_model.ActionRun{ID: oldSuccess.ID})
		unittest.AssertNotExistsBean(t, &actions_model.ActionRun{ID: oldFailure.ID})
		unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: recent.ID})
		unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: oldRunning.ID})
		unittest.AssertNotExistsBean(t, &actions_model.ActionRun{ID: oldCanceled.ID})
		unittest.AssertNotExistsBean(t, &actions_model.ActionRun{ID: oldSkipped.ID})
	})
	t.Run("error during deleting", func(t *testing.T) {
		defer test.MockVariableValue(&cleanupOldRunsBatchSize, 3)()
		defer test.MockVariableValue(&setting.IsInTesting, false)() // skip the panic
		deleteAllRuns(t)

		for i := range int64(6) {
			insertCleanupRun(t, 3000+i, actions_model.StatusSuccess, old)
		}
		var deletedIndices []int64
		deleteRun := func(ctx context.Context, run *actions_model.ActionRun) error {
			if run.Index%2 == 0 {
				return errors.New("some error")
			}
			deletedIndices = append(deletedIndices, run.Index)
			_, err := db.DeleteByID[actions_model.ActionRun](ctx, run.ID)
			return err
		}
		total, err := cleanupOldRuns(t.Context(), now, []actions_model.Status{actions_model.StatusSuccess}, deleteRun)
		require.NoError(t, err)
		// 3000/3002/3004 keep failing and fill up the batch, so the loop stops after 3001 and 3003
		assert.Equal(t, 2, total)
		assert.Equal(t, []int64{3001, 3003}, deletedIndices)
	})
}

func TestCleanupRetentionZeroKeepsForever(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	defer test.MockVariableValue(&setting.Actions.LogRetentionDays, 0)()
	defer test.MockVariableValue(&setting.Actions.ArtifactRetentionDays, 0)()

	liveLogs := unittest.Cond("stopped > 0 AND log_expired = ?", false)

	t.Run("logs", func(t *testing.T) {
		before := unittest.GetCount(t, &actions_model.ActionTask{}, liveLogs)
		require.Positive(t, before)
		require.NoError(t, CleanupExpiredLogs(t.Context()))
		assert.Equal(t, before, unittest.GetCount(t, &actions_model.ActionTask{}, liveLogs))
	})

	t.Run("artifacts", func(t *testing.T) {
		task := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 47})
		art, err := actions_model.CreateArtifact(t.Context(), task, "never-expires", "a.txt", optional.None[timeutil.TimeStamp]())
		require.NoError(t, err)
		assert.Zero(t, art.ExpiredUnix)

		// a workflow-requested expiry is honored, only the instance default may mean never
		asked, err := actions_model.CreateArtifact(t.Context(), task, "client-asked", "b.txt", optional.Some(timeutil.TimeStampNow()))
		require.NoError(t, err)
		assert.Positive(t, asked.ExpiredUnix)

		// re-uploading refreshes the expiry and returns it
		reuploaded, err := actions_model.CreateArtifact(t.Context(), task, "client-asked", "b.txt", optional.None[timeutil.TimeStamp]())
		require.NoError(t, err)
		assert.Zero(t, reuploaded.ExpiredUnix)

		// a past expiry must stay reapable, not land on the sentinel
		past, err := actions_model.CreateArtifact(t.Context(), task, "long-gone", "c.txt", optional.Some(timeutil.TimeStamp(-1000)))
		require.NoError(t, err)
		assert.Positive(t, past.ExpiredUnix)

		_, err = db.GetEngine(t.Context()).In("id", art.ID, past.ID).Cols("status").
			Update(&actions_model.ActionArtifact{Status: actions_model.ArtifactStatusUploadConfirmed})
		require.NoError(t, err)
		expiring, err := actions_model.ListNeedExpiredArtifacts(t.Context())
		require.NoError(t, err)
		ids := container.FilterSlice(expiring, func(a *actions_model.ActionArtifact) (int64, bool) { return a.ID, true })
		assert.NotContains(t, ids, art.ID)
		assert.Contains(t, ids, past.ID)
	})
}
