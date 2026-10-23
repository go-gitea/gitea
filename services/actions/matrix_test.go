// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"fmt"
	"strings"
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
    runs-on: ubuntu-latest
    steps: [{run: echo}]
  build:
    needs: generate
    runs-on: ubuntu-latest
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
	run.LatestAttemptID = attempt.ID
	require.NoError(t, actions_model.UpdateRun(ctx, run, "latest_attempt_id"))

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
	build.DeferredMatrixPayload = payload
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
			assert.Empty(t, sibling.DeferredMatrixPayload, "only the placeholder anchors the group with the raw payload")
		}
		assert.ElementsMatch(t, []string{"build (a)", "build (b)", "build (c)"}, names)

		reloaded := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: job.ID})
		assert.Equal(t, "build (a)", reloaded.Name)
		assert.False(t, reloaded.IsMatrixDeferred)
		assert.NotEmpty(t, reloaded.DeferredMatrixPayload, "the claim must not erase the raw payload")
		reloaded.MaxParallel = 3
		require.NoError(t, restoreDeferredMatrixPlaceholder(reloaded))
		assert.Equal(t, "build", reloaded.Name)
		assert.Zero(t, reloaded.MaxParallel)
		assert.True(t, reloaded.IsMatrixDeferred)
	})

	// A matrix that can never produce runnable combinations fails the job instead of rolling the
	// emitter pass back, so it is not retried forever.
	for _, tt := range []struct {
		name, matrixValue string
		outputs           map[string]string
		prepare           func(t *testing.T, job *actions_model.ActionRunJob)
	}{
		{name: "unresolvable matrix", matrixValue: "${{ fromJson(needs.generate.outputs.missing) }}"},
		{
			name:        "expansion exceeding the per-attempt job limit",
			matrixValue: "${{ fromJson(needs.generate.outputs.many) }}",
			// One combination more than the attempt has room for: the cap is MaxJobNumPerRun less
			// the rows the setup plants, plus the placeholder the first combination reuses.
			outputs: map[string]string{"many": "[" + strings.Repeat("0,", actions_model.MaxJobNumPerRun-2) + "0]"},
		},
		{
			// A malformed payload fails the same way on every pass, so returning it would requeue the
			// run forever instead of ever reaching a terminal status.
			name:        "unreadable caller inputs",
			matrixValue: "${{ fromJson(needs.generate.outputs.values) }}",
			prepare: func(t *testing.T, job *actions_model.ActionRunJob) {
				_, err := db.Exec(t.Context(), "UPDATE `action_run_job` SET call_payload = ? WHERE id = ?", "{", job.ParentJobID)
				require.NoError(t, err)
			},
		},
	} {
		t.Run(tt.name+" fails the job", func(t *testing.T) {
			outputs := tt.outputs
			if outputs == nil {
				outputs = map[string]string{"values": `["a","b","c"]`}
			}
			job := setupDeferredMatrixJob(t, tt.matrixValue, outputs)
			if tt.prepare != nil {
				tt.prepare(t, job)
			}

			siblings, err := expandDeferredMatrix(t.Context(), job, nil)
			require.NoError(t, err)
			assert.Empty(t, siblings)
			assert.Equal(t, actions_model.StatusFailure, job.Status)
			assert.True(t, job.IsMatrixDeferred, "the flag survives, marking the payload as still unexpanded for reruns")
			assert.NotZero(t, job.Stopped)
			// A rejected expansion must not leave the first combination's name behind.
			assert.Equal(t, "build", unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: job.ID}).Name)
		})
	}
}

func TestDeferredMatrixResolverSkipsWithoutSucceededNeeds(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	for _, needStatus := range []actions_model.Status{actions_model.StatusFailure, actions_model.StatusSkipped} {
		t.Run(needStatus.String(), func(t *testing.T) {
			ctx := t.Context()
			job := setupDeferredMatrixJob(t, "${{ fromJson(needs.generate.outputs.values) }}", nil)
			_, err := db.Exec(ctx, "UPDATE `action_run_job` SET status = ? WHERE run_id = ? AND job_id = ?", int(needStatus), job.RunID, "generate")
			require.NoError(t, err)

			jobs := runJobs(t, job.RunID, job.RunAttemptID)
			require.NoError(t, jobs.LoadRuns(ctx, false))

			updates, err := newJobStatusResolver(jobs, nil).Resolve(ctx)
			require.NoError(t, err)
			assert.Equal(t, actions_model.StatusSkipped, updates[job.ID])

			var names []string
			for _, runJob := range runJobs(t, job.RunID, job.RunAttemptID) {
				if runJob.JobID == "build" {
					names = append(names, runJob.Name)
				}
			}
			assert.Equal(t, []string{"build"}, names)
		})
	}
}

func TestDeferredMatrixResolverDefersDependents(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	build := setupDeferredMatrixJob(t, "${{ fromJson(needs.generate.outputs.values) }}", map[string]string{"values": `["a","b"]`})

	attemptJobID, err := actions_model.GetNextAttemptJobID(ctx, build.RunID)
	require.NoError(t, err)
	report := &actions_model.ActionRunJob{
		RunID: build.RunID, RunAttemptID: build.RunAttemptID, AttemptJobID: attemptJobID,
		RepoID: build.RepoID, OwnerID: build.OwnerID, ParentJobID: build.ParentJobID,
		JobID: "report", Name: "report", Status: actions_model.StatusBlocked,
		Needs: []string{"build"}, WorkflowPayload: minimalWorkflowPayload("report"),
	}
	require.NoError(t, db.Insert(ctx, report))

	jobs := runJobs(t, build.RunID, build.RunAttemptID)
	require.NoError(t, jobs.LoadRuns(ctx, false))
	updates, err := newJobStatusResolver(jobs, nil).Resolve(ctx)
	require.NoError(t, err)

	assert.Equal(t, actions_model.StatusWaiting, updates[build.ID], "the placeholder runs as `build (a)`")
	assert.NotContains(t, updates, report.ID, "report must wait for the re-emit, which sees `build (b)` too")

	// The sibling the pass would otherwise have resolved report against is there, and still to run.
	sibling := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{RunID: build.RunID, Name: "build (b)"})
	assert.Equal(t, actions_model.StatusBlocked, sibling.Status)
}
