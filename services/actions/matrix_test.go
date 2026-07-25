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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testRunIndex hands out the per-repo run index, which is unique per action_run row.
var testRunIndex int64 = 9100

// setupDeferredMatrixJob plants a completed `generate` job exposing outputs and the blocked `build`
// placeholder that depends on them, and returns the placeholder. Both are children of a reusable
// workflow caller, the case where a sibling losing ParentJobID would break needs resolution.
func setupDeferredMatrixJob(t *testing.T, matrixValue string, outputs map[string]string) *actions_model.ActionRunJob {
	t.Helper()
	ctx := t.Context()

	// The `build` job takes its matrix from `generate`'s outputs, so Parse defers it.
	workflows, err := jobparser.Parse(fmt.Appendf(nil, `
on: push
jobs:
  generate:
    steps: [{run: echo}]
  build:
    needs: generate
    strategy:
      matrix:
        value: %s
    steps: [{run: echo}]
`, matrixValue))
	require.NoError(t, err)
	var placeholder *jobparser.SingleWorkflow
	for _, workflow := range workflows {
		if id, _ := workflow.Job(); id == "build" {
			placeholder = workflow
		}
	}
	require.NotNil(t, placeholder, "the build job must be planned as a single deferred placeholder")
	id, job := placeholder.Job()
	require.NoError(t, placeholder.SetJob(id, job.EraseNeeds()))
	payload, err := placeholder.Marshal()
	require.NoError(t, err)

	testRunIndex++
	run := &actions_model.ActionRun{
		RepoID: 4, OwnerID: 1, TriggerUserID: 1, Index: testRunIndex,
		WorkflowID: "dynamic.yml", Ref: "refs/heads/main", Status: actions_model.StatusRunning,
	}
	require.NoError(t, db.Insert(ctx, run))
	attempt := &actions_model.ActionRunAttempt{RepoID: 4, RunID: run.ID, Attempt: 1, Status: actions_model.StatusRunning}
	require.NoError(t, db.Insert(ctx, attempt))

	// Needs outputs are read straight from action_task_output, so no action_task row is required;
	// the run's id doubles as a task id that is unique across subtests.
	taskID := run.ID
	for key, value := range outputs {
		require.NoError(t, db.Insert(ctx, &actions_model.ActionTaskOutput{TaskID: taskID, OutputKey: key, OutputValue: value}))
	}

	newJob := func(jobID string) *actions_model.ActionRunJob {
		attemptJobID, err := actions_model.GetNextAttemptJobID(ctx, run.ID)
		require.NoError(t, err)
		return &actions_model.ActionRunJob{
			RunID: run.ID, RunAttemptID: attempt.ID, AttemptJobID: attemptJobID, RepoID: 4, OwnerID: 1,
			JobID: jobID, Name: jobID, Status: actions_model.StatusRunning,
		}
	}
	caller := newJob("call")
	caller.IsReusableCaller, caller.IsExpanded = true, true
	require.NoError(t, db.Insert(ctx, caller))

	generate := newJob("generate")
	generate.ParentJobID, generate.Status, generate.TaskID = caller.ID, actions_model.StatusSuccess, taskID
	require.NoError(t, db.Insert(ctx, generate))

	build := newJob("build")
	build.ParentJobID, build.Status = caller.ID, actions_model.StatusBlocked
	build.Needs, build.WorkflowPayload, build.IsMatrixDeferred = []string{"generate"}, payload, true
	// Values a sibling must inherit rather than silently reset.
	build.WorkflowSourceRepoID, build.WorkflowSourceCommitSHA = 42, "abc123"
	require.NoError(t, db.Insert(ctx, build))
	return build
}

func TestExpandDeferredMatrix(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	t.Run("expands into siblings", func(t *testing.T) {
		job := setupDeferredMatrixJob(t, "${{ fromJson(needs.generate.outputs.values) }}", map[string]string{"values": `["a","b","c"]`})

		siblings, err := expandDeferredMatrix(t.Context(), job, nil)
		require.NoError(t, err)
		require.Len(t, siblings, 2)

		// The placeholder is reused as the first combination and stays blocked for the `if:` gate.
		assert.Equal(t, "build (a)", job.Name)
		assert.False(t, job.IsMatrixDeferred)
		assert.Equal(t, actions_model.StatusBlocked, job.Status)

		names := []string{job.Name}
		for _, sibling := range siblings {
			names = append(names, sibling.Name)
			assert.Equal(t, actions_model.StatusBlocked, sibling.Status)
			assert.False(t, sibling.IsMatrixDeferred, "a sibling must not be expanded again")
			assert.Equal(t, []string{"generate"}, sibling.Needs)
			assert.Equal(t, job.ParentJobID, sibling.ParentJobID, "needs resolution is scoped by ParentJobID")
			assert.Equal(t, int64(42), sibling.WorkflowSourceRepoID)
			assert.Equal(t, "abc123", sibling.WorkflowSourceCommitSHA)
			assert.Greater(t, sibling.AttemptJobID, job.AttemptJobID, "siblings take fresh ids from the run-wide counter")
		}
		assert.ElementsMatch(t, []string{"build (a)", "build (b)", "build (c)"}, names)

		reloaded := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: job.ID})
		assert.Equal(t, "build (a)", reloaded.Name)
		assert.False(t, reloaded.IsMatrixDeferred)
	})

	// A matrix that can never produce runnable combinations fails the job instead of rolling the
	// emitter pass back, so it is not retried forever.
	for _, tt := range []struct {
		name, matrixValue string
		prepare           func(t *testing.T, job *actions_model.ActionRunJob)
	}{
		{name: "unresolvable matrix", matrixValue: "${{ fromJson(needs.generate.outputs.missing) }}"},
		{
			name:        "expansion exceeding the per-run job limit",
			matrixValue: "${{ fromJson(needs.generate.outputs.values) }}",
			prepare: func(t *testing.T, job *actions_model.ActionRunJob) {
				_, err := db.Exec(t.Context(), "UPDATE `action_run_attempt_job_id_index` SET max_index = ? WHERE group_id = ?", actions_model.MaxJobNumPerRun-1, job.RunID)
				require.NoError(t, err)
			},
		},
	} {
		t.Run(tt.name+" fails the job", func(t *testing.T) {
			job := setupDeferredMatrixJob(t, tt.matrixValue, map[string]string{"values": `["a","b","c"]`})
			if tt.prepare != nil {
				tt.prepare(t, job)
			}

			siblings, err := expandDeferredMatrix(t.Context(), job, nil)
			require.NoError(t, err)
			assert.Empty(t, siblings)
			assert.Equal(t, actions_model.StatusFailure, job.Status)
			assert.False(t, job.IsMatrixDeferred)
			assert.NotZero(t, job.Stopped)
			// A rejected expansion must not leave the first combination's name behind.
			assert.Equal(t, "build", unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: job.ID}).Name)
		})
	}
}

// TestDeferredMatrixWithUnsuccessfulNeed covers the resolver deciding the job's fate before touching
// the matrix: a need that did not succeed leaves no outputs to build it from, so the placeholder has
// to be skipped like any other job with such a need, not failed over a matrix it never had to
// evaluate. Expanding first would also insert combinations that can only be skipped.
func TestDeferredMatrixWithUnsuccessfulNeed(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	for _, tt := range []struct {
		name       string
		needStatus actions_model.Status
	}{
		{"failed need", actions_model.StatusFailure},
		{"skipped need", actions_model.StatusSkipped},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			job := setupDeferredMatrixJob(t, "${{ fromJson(needs.generate.outputs.values) }}", nil)
			_, err := db.Exec(ctx, "UPDATE `action_run_job` SET status = ? WHERE run_id = ? AND job_id = ?", int(tt.needStatus), job.RunID, "generate")
			require.NoError(t, err)

			jobs, err := actions_model.GetRunJobsByRunAndAttemptID(ctx, job.RunID, job.RunAttemptID)
			require.NoError(t, err)
			run, err := actions_model.GetRunByRepoAndID(ctx, job.RepoID, job.RunID)
			require.NoError(t, err)
			for _, runJob := range jobs {
				runJob.Run = run
			}

			updates, err := newJobStatusResolver(jobs, nil).Resolve(ctx)
			require.NoError(t, err)
			assert.Equal(t, actions_model.StatusSkipped, updates[job.ID])

			// The matrix was never evaluated, so no combination was inserted.
			after, err := actions_model.GetRunJobsByRunAndAttemptID(ctx, job.RunID, job.RunAttemptID)
			require.NoError(t, err)
			assert.Len(t, after, len(jobs))
		})
	}
}
