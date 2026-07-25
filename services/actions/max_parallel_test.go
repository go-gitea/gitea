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

func TestParseMaxParallel(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"3", 3},
		{"0", 0},
		{"-1", 0},
		{"1.5", 1},           // GitHub casts the YAML number to int
		{"-1.5", 0},          // truncates to -1, which means unlimited
		{"1e3", 256},         // clamped to MaxJobNumPerRun
		{"nan", 0},           // must not reach the int cast
		{"${{ vars.n }}", 0}, // expressions are not evaluated yet
		{"abc", 0},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, parseMaxParallel("job", tt.input), "input %q", tt.input)
	}
}

// a 5-job matrix capped at 2, so 2 jobs start and 3 stay blocked
const maxParallelWorkflow = `name: max-parallel
on: push
jobs:
  build:
    runs-on: ubuntu-latest
    strategy:
      max-parallel: 2
      matrix:
        version: [1, 2, 3, 4, 5]
    steps:
      - run: echo hi
`

func insertMaxParallelRun(t *testing.T, needApproval bool) *actions_model.ActionRun {
	t.Helper()
	run := &actions_model.ActionRun{
		Title:             "max-parallel",
		RepoID:            4,
		OwnerID:           1,
		WorkflowID:        "max-parallel.yaml",
		TriggerUserID:     1,
		Ref:               "refs/heads/master",
		CommitSHA:         "c2d72f548424103f01ee1dc02889c1e2bff816b0",
		Event:             "push",
		TriggerEvent:      "push",
		EventPayload:      "{}",
		NeedApproval:      needApproval,
		WorkflowRepoID:    4,
		WorkflowCommitSHA: "c2d72f548424103f01ee1dc02889c1e2bff816b0",
	}
	require.NoError(t, PrepareRunAndInsert(t.Context(), []byte(maxParallelWorkflow), run, nil))
	return run
}

func runJobs(t *testing.T, runID, attemptID int64) actions_model.ActionJobList {
	t.Helper()
	jobs, err := actions_model.GetRunJobsByRunAndAttemptID(t.Context(), runID, attemptID)
	require.NoError(t, err)
	return jobs
}

func statusCounts(jobs actions_model.ActionJobList) map[actions_model.Status]int {
	counts := map[actions_model.Status]int{}
	for _, job := range jobs {
		counts[job.Status]++
	}
	return counts
}

func TestPrepareRunAndInsert_MaxParallel(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	run := insertMaxParallelRun(t, false)
	jobs := runJobs(t, run.ID, run.LatestAttemptID)
	assert.Equal(t, map[actions_model.Status]int{
		actions_model.StatusWaiting: 2,
		actions_model.StatusBlocked: 3,
	}, statusCounts(jobs))

	for _, job := range jobs {
		assert.Equal(t, 2, job.MaxParallel, "every matrix sibling carries the limit")
	}
}

// A reusable workflow declares its own strategy, so the limit must reach the child jobs.
func TestInsertCallerChildren_MaxParallel(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	run := &actions_model.ActionRun{
		RepoID: 4, OwnerID: 1, Index: 9701, TriggerUserID: 1, TriggerEvent: "push", EventPayload: "{}",
	}
	require.NoError(t, db.Insert(ctx, run))
	require.NoError(t, run.LoadAttributes(ctx))

	attempt := &actions_model.ActionRunAttempt{RepoID: run.RepoID, RunID: run.ID, Attempt: 1}
	require.NoError(t, db.Insert(ctx, attempt))

	caller := &actions_model.ActionRunJob{
		RunID: run.ID, RunAttemptID: attempt.ID, RepoID: run.RepoID, OwnerID: run.OwnerID,
		JobID: "caller", AttemptJobID: 1, IsReusableCaller: true,
	}
	require.NoError(t, db.Insert(ctx, caller))

	called := []byte(`name: called
on: workflow_call
jobs:
  build:
    runs-on: ubuntu-latest
    strategy:
      max-parallel: 2
      matrix:
        version: [1, 2, 3]
    steps:
      - run: echo hi
`)
	require.NoError(t, insertCallerChildren(ctx, run, attempt, caller, called, run.RepoID, run.CommitSHA, nil, nil))

	children := 0
	for _, job := range runJobs(t, run.ID, attempt.ID) {
		if job.ParentJobID == caller.ID {
			children++
			assert.Equal(t, 2, job.MaxParallel)
		}
	}
	assert.Equal(t, 3, children)
}

// An approval-gated run inserts every job as Blocked, so the cap has to be applied on approval.
func TestApproveRuns_MaxParallel(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	run := insertMaxParallelRun(t, true)
	assert.Equal(t, map[actions_model.Status]int{actions_model.StatusBlocked: 5}, statusCounts(runJobs(t, run.ID, run.LatestAttemptID)))

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: run.RepoID})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	require.NoError(t, ApproveRuns(t.Context(), repo, doer, []int64{run.ID}))

	assert.Equal(t, map[actions_model.Status]int{
		actions_model.StatusWaiting: 2,
		actions_model.StatusBlocked: 3,
	}, statusCounts(runJobs(t, run.ID, run.LatestAttemptID)))
}
