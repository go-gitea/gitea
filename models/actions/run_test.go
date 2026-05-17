// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func TestUpdateRepoRunsNumbers(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// update the number to a wrong one, the original is 3
	_, err := db.GetEngine(t.Context()).ID(4).Cols("num_closed_action_runs").Update(&repo_model.Repository{
		NumClosedActionRuns: 2,
	})
	assert.NoError(t, err)

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	assert.Equal(t, 4, repo.NumActionRuns)
	assert.Equal(t, 2, repo.NumClosedActionRuns)

	// now update will correct them, only num_actionr_runs and num_closed_action_runs should be updated
	UpdateRepoRunsNumbers(t.Context(), repo.ID)
	repo = unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	assert.Equal(t, 4, repo.NumActionRuns)
	assert.Equal(t, 3, repo.NumClosedActionRuns)
}

func TestActionRun_Duration_NonNegative(t *testing.T) {
	run := &ActionRun{
		Started:          timeutil.TimeStamp(100),
		Stopped:          timeutil.TimeStamp(200),
		Status:           StatusSuccess,
		PreviousDuration: -time.Hour,
	}
	assert.Equal(t, time.Duration(0), run.Duration())
}

func TestInsertActionRunJobs(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	t.Run("empty slice is a no-op", func(t *testing.T) {
		assert.NoError(t, InsertActionRunJobs(t.Context(), nil))
		assert.NoError(t, InsertActionRunJobs(t.Context(), []*ActionRunJob{}))
	})

	t.Run("bulk-insert multiple rows", func(t *testing.T) {
		// Use distinct, non-colliding RunID/Repo so we don't tangle with fixtures.
		const runID = int64(987654)
		batch := []*ActionRunJob{
			{RunID: runID, RepoID: 1, OwnerID: 1, Name: "batch-a", JobID: "j", Status: StatusWaiting, MatrixValues: map[string]any{"k": "a"}},
			{RunID: runID, RepoID: 1, OwnerID: 1, Name: "batch-b", JobID: "j", Status: StatusWaiting, MatrixValues: map[string]any{"k": "b"}},
			{RunID: runID, RepoID: 1, OwnerID: 1, Name: "batch-c", JobID: "j", Status: StatusWaiting, MatrixValues: map[string]any{"k": "c"}},
		}
		assert.NoError(t, InsertActionRunJobs(t.Context(), batch))

		var loaded []*ActionRunJob
		assert.NoError(t, db.GetEngine(t.Context()).Where("run_id = ?", runID).OrderBy("id").Find(&loaded))
		assert.Len(t, loaded, 3)
		assert.Equal(t, "batch-a", loaded[0].Name)
		assert.Equal(t, "batch-c", loaded[2].Name)
		// MatrixValues round-trip through the JSON column.
		assert.Equal(t, "a", loaded[0].MatrixValues["k"])
		assert.Equal(t, "c", loaded[2].MatrixValues["k"])
	})
}
