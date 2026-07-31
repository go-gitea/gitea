// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApproveRuns(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	insertRun := func(index int64, status actions_model.Status, needApproval bool, approvedBy int64) *actions_model.ActionRun {
		run := &actions_model.ActionRun{
			Title: "approve-run", RepoID: repo.ID, OwnerID: repo.OwnerID, WorkflowID: "test.yaml", Index: index,
			TriggerUserID: doer.ID, Ref: "refs/heads/main",
			CommitSHA: "c2d72f548424103f01ee1dc02889c1e2bff816b0", Event: "push", TriggerEvent: "push",
			Status: status, NeedApproval: needApproval, ApprovedBy: approvedBy,
		}
		require.NoError(t, db.Insert(t.Context(), run))
		return run
	}
	insertJob := func(run *actions_model.ActionRun, status actions_model.Status) *actions_model.ActionRunJob {
		job := &actions_model.ActionRunJob{
			RunID: run.ID, RepoID: run.RepoID, OwnerID: run.OwnerID, CommitSHA: run.CommitSHA,
			Name: "job1", Attempt: 1, JobID: "job1", Status: status,
			RunsOn: []string{"ubuntu-latest"},
		}
		require.NoError(t, db.Insert(t.Context(), job))
		return job
	}

	t.Run("approve blocked run", func(t *testing.T) {
		run := insertRun(1001, actions_model.StatusBlocked, true, 0)
		job := insertJob(run, actions_model.StatusBlocked)

		require.NoError(t, ApproveRuns(t.Context(), repo, doer, []int64{run.ID}))

		run = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run.ID})
		assert.False(t, run.NeedApproval)
		assert.Equal(t, doer.ID, run.ApprovedBy)
		assert.Equal(t, actions_model.StatusWaiting, unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: job.ID}).Status)
	})

	t.Run("re-approving an approved run is a no-op", func(t *testing.T) {
		run := insertRun(1002, actions_model.StatusRunning, false, 4)
		job := insertJob(run, actions_model.StatusRunning)

		require.NoError(t, ApproveRuns(t.Context(), repo, doer, []int64{run.ID}))

		run = unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRun{ID: run.ID})
		assert.EqualValues(t, 4, run.ApprovedBy, "approver must not be overwritten")
		assert.Equal(t, actions_model.StatusRunning, unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: job.ID}).Status, "started job must not be reset to waiting")
	})
}
