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

func TestUnapprovedRunDoesNotCancelConcurrencyPeers(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	previousRun := &actions_model.ActionRun{RepoID: 4, OwnerID: 1, Index: 9001, TriggerUserID: 1, TriggerEvent: "push", EventPayload: "{}", Status: actions_model.StatusWaiting}
	require.NoError(t, db.Insert(ctx, previousRun))
	previousAttempt := &actions_model.ActionRunAttempt{RepoID: 4, RunID: previousRun.ID, Attempt: 1, Status: actions_model.StatusWaiting, ConcurrencyGroup: "shared", ConcurrencyCancel: true}
	require.NoError(t, db.Insert(ctx, previousAttempt))
	previousJob := &actions_model.ActionRunJob{RunID: previousRun.ID, RunAttemptID: previousAttempt.ID, RepoID: 4, OwnerID: 1, JobID: "deploy", AttemptJobID: 1, Status: actions_model.StatusWaiting}
	require.NoError(t, db.Insert(ctx, previousJob))

	content := []byte(`on: pull_request
concurrency:
  group: shared
  cancel-in-progress: true
jobs:
  deploy:
    runs-on: ubuntu-latest
    concurrency:
      group: shared
      cancel-in-progress: true
    steps:
      - run: echo hi
`)
	run := &actions_model.ActionRun{
		RepoID: 4, OwnerID: 1, WorkflowID: "test.yaml", TriggerUserID: 2,
		Ref: "refs/pull/1/head", CommitSHA: "c2d72f548424103f01ee1dc02889c1e2bff816b0",
		Event: "pull_request", TriggerEvent: "pull_request", EventPayload: "{}", NeedApproval: true,
		WorkflowRepoID: 4, WorkflowCommitSHA: "c2d72f548424103f01ee1dc02889c1e2bff816b0",
	}
	require.NoError(t, PrepareRunAndInsert(ctx, content, run, nil))

	assert.Equal(t, actions_model.StatusWaiting, unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: previousJob.ID}).Status)

	oldEmitJobsIfReadyByRun := EmitJobsIfReadyByRun
	EmitJobsIfReadyByRun = func(int64) error { return nil }
	t.Cleanup(func() { EmitJobsIfReadyByRun = oldEmitJobsIfReadyByRun })

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	_, err := ApproveRuns(ctx, repo, doer, []int64{run.ID})
	require.NoError(t, err)
	assert.Equal(t, actions_model.StatusCancelled, unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: previousJob.ID}).Status)
	jobs := runJobs(t, run.ID, run.LatestAttemptID)
	require.Len(t, jobs, 1)
	assert.Equal(t, actions_model.StatusWaiting, jobs[0].Status)
}
