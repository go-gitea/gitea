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

	insertRun := func(index int64) *actions_model.ActionRun {
		run := &actions_model.ActionRun{
			Title: "approve-run", RepoID: repo.ID, OwnerID: repo.OwnerID, WorkflowID: "test.yaml", Index: index,
			TriggerUserID: doer.ID, Ref: "refs/heads/main",
			CommitSHA: "c2d72f548424103f01ee1dc02889c1e2bff816b0", Event: "push", TriggerEvent: "push",
			Status: actions_model.StatusBlocked, NeedApproval: true,
		}
		require.NoError(t, db.Insert(t.Context(), run))
		return run
	}
	insertJob := func(run *actions_model.ActionRun, needs ...string) *actions_model.ActionRunJob {
		job := &actions_model.ActionRunJob{
			RunID: run.ID, RepoID: run.RepoID, OwnerID: run.OwnerID, CommitSHA: run.CommitSHA,
			Name: "job1", Attempt: 1, JobID: "job1", Status: actions_model.StatusBlocked,
			RunsOn: []string{"ubuntu-latest"}, Needs: needs,
		}
		require.NoError(t, db.Insert(t.Context(), job))
		return job
	}

	t.Run("approve unblocks a job with no dependencies", func(t *testing.T) {
		run := insertRun(1001)
		job := insertJob(run)

		approved, err := ApproveRuns(t.Context(), repo, doer, []int64{run.ID})
		require.NoError(t, err)
		require.Len(t, approved, 1)
		assert.False(t, approved[0].NeedApproval)
		assert.Equal(t, doer.ID, approved[0].ApprovedBy)

		assert.Equal(t, actions_model.StatusWaiting, unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: job.ID}).Status)
	})

	t.Run("a job with unmet dependencies stays blocked", func(t *testing.T) {
		run := insertRun(1002)
		job := insertJob(run, "some-other-job")

		approved, err := ApproveRuns(t.Context(), repo, doer, []int64{run.ID})
		require.NoError(t, err)
		require.Len(t, approved, 1)
		assert.False(t, approved[0].NeedApproval, "run itself is still marked approved")

		assert.Equal(t, actions_model.StatusBlocked, unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: job.ID}).Status, "job with unmet needs must stay blocked until job_emitter unblocks it")
	})

	t.Run("approving several runs returns them in the requested order", func(t *testing.T) {
		run1 := insertRun(1003)
		insertJob(run1)
		run2 := insertRun(1004)
		insertJob(run2)

		approved, err := ApproveRuns(t.Context(), repo, doer, []int64{run2.ID, run1.ID})
		require.NoError(t, err)
		require.Len(t, approved, 2)
		assert.Equal(t, run2.ID, approved[0].ID)
		assert.Equal(t, run1.ID, approved[1].ID)
		assert.False(t, approved[0].NeedApproval)
		assert.False(t, approved[1].NeedApproval)
	})
}
