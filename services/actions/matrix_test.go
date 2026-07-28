// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"fmt"
	"testing"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/actions/jobparser"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testRunIndex hands out the per-repo run index, which is unique per action_run row.
var testRunIndex int64 = 9100

// setupDeferredMatrixJob plants a completed `generate` job exposing outputs and the blocked `build`
// placeholder that depends on them, and returns the placeholder. Both are children of a reusable
// workflow caller, the case where a sibling losing ParentJobID would break needs resolution.
// jobIf is the `build` job's `if:` expression, omitted entirely when empty.
func setupDeferredMatrixJob(t *testing.T, matrixValue, jobIf string, outputs map[string]string) *actions_model.ActionRunJob {
	t.Helper()
	ctx := t.Context()

	ifLine := ""
	if jobIf != "" {
		ifLine = "    if: " + jobIf + "\n"
	}
	// The `build` job takes its matrix from `generate`'s outputs, so Parse defers it.
	workflows, err := jobparser.Parse(fmt.Appendf(nil, `
on: push
jobs:
  generate:
    steps: [{run: echo}]
  build:
    needs: generate
%s    strategy:
      matrix:
        value: %s
    steps: [{run: echo}]
`, ifLine, matrixValue))
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
		job := setupDeferredMatrixJob(t, "${{ fromJson(needs.generate.outputs.values) }}", "", map[string]string{"values": `["a","b","c"]`})

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
			assert.Empty(t, sibling.DeferredMatrixPayload, "only the placeholder anchors the group with the raw payload")
		}
		assert.ElementsMatch(t, []string{"build (a)", "build (b)", "build (c)"}, names)

		reloaded := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: job.ID})
		assert.Equal(t, "build (a)", reloaded.Name)
		assert.False(t, reloaded.IsMatrixDeferred)
		assert.NotEmpty(t, reloaded.DeferredMatrixPayload, "the claim must not erase the raw payload")
		assertSingleMatrixAnchor(t, job.RunID, job.RunAttemptID)
	})

	// A matrix that can never produce runnable combinations fails the job instead of rolling the
	// emitter pass back, so it is not retried forever.
	for _, tt := range []struct {
		name, matrixValue string
		prepare           func(t *testing.T, job *actions_model.ActionRunJob)
	}{
		{name: "unresolvable matrix", matrixValue: "${{ fromJson(needs.generate.outputs.missing) }}"},
		{
			name:        "expansion exceeding the per-attempt job limit",
			matrixValue: "${{ fromJson(needs.generate.outputs.values) }}",
			prepare: func(t *testing.T, job *actions_model.ActionRunJob) {
				existing, err := actions_model.CountRunJobsByRunAndAttemptID(t.Context(), job.RunID, job.RunAttemptID)
				require.NoError(t, err)
				fillers := make([]*actions_model.ActionRunJob, 0, actions_model.MaxJobNumPerRun-1-existing)
				for i := existing; i < actions_model.MaxJobNumPerRun-1; i++ {
					fillers = append(fillers, &actions_model.ActionRunJob{
						RunID: job.RunID, RunAttemptID: job.RunAttemptID, AttemptJobID: 1000 + i,
						RepoID: job.RepoID, OwnerID: job.OwnerID,
						JobID: fmt.Sprintf("filler%d", i), Name: fmt.Sprintf("filler%d", i),
						Status: actions_model.StatusSuccess,
					})
				}
				require.NoError(t, db.Insert(t.Context(), fillers))
			},
		},
	} {
		t.Run(tt.name+" fails the job", func(t *testing.T) {
			job := setupDeferredMatrixJob(t, tt.matrixValue, "", map[string]string{"values": `["a","b","c"]`})
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
			job := setupDeferredMatrixJob(t, "${{ fromJson(needs.generate.outputs.values) }}", "", nil)
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

// TestDeferredMatrixIfIsGatedPerCombination covers the placeholder reused as the first combination
// being gated by its own `matrix.*`: the `if:` decided before expansion could only see the raw
// matrix expression, so a combination the condition excludes would otherwise run just because it
// happened to be the one inheriting the placeholder row.
func TestDeferredMatrixIfIsGatedPerCombination(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	job := setupDeferredMatrixJob(t, "${{ fromJson(needs.generate.outputs.values) }}",
		"${{ matrix.value != 'a' }}", map[string]string{"values": `["a","b"]`})

	jobs, err := actions_model.GetRunJobsByRunAndAttemptID(ctx, job.RunID, job.RunAttemptID)
	require.NoError(t, err)
	run, err := actions_model.GetRunByRepoAndID(ctx, job.RepoID, job.RunID)
	require.NoError(t, err)
	for _, runJob := range jobs {
		runJob.Run = run
	}

	updates, err := newJobStatusResolver(jobs, nil).Resolve(ctx)
	require.NoError(t, err)
	assert.Equal(t, actions_model.StatusSkipped, updates[job.ID], "`build (a)` is excluded by its own `if:`")

	// The matrix was still expanded, so `build (b)` exists and is left for the next pass to resolve.
	after, err := actions_model.GetRunJobsByRunAndAttemptID(ctx, job.RunID, job.RunAttemptID)
	require.NoError(t, err)
	var names []string
	for _, runJob := range after {
		if runJob.JobID == "build" {
			names = append(names, runJob.Name)
		}
	}
	assert.ElementsMatch(t, []string{"build (a)", "build (b)"}, names)
}

// finishJobWithOutputs marks a job successful with a fresh task exposing the given outputs.
func finishJobWithOutputs(t *testing.T, job *actions_model.ActionRunJob, taskID int64, outputs map[string]string) {
	t.Helper()
	ctx := t.Context()
	for key, value := range outputs {
		require.NoError(t, db.Insert(ctx, &actions_model.ActionTaskOutput{TaskID: taskID, OutputKey: key, OutputValue: value}))
	}
	_, err := db.Exec(ctx, "UPDATE `action_run_job` SET status = ?, task_id = ? WHERE id = ?",
		int(actions_model.StatusSuccess), taskID, job.ID)
	require.NoError(t, err)
}

// assertSingleMatrixAnchor pins the invariant the rerun collapse relies on:
// within one attempt, exactly one row of a dynamic-matrix job carries the raw payload.
func assertSingleMatrixAnchor(t *testing.T, runID, attemptID int64) {
	t.Helper()
	jobs, err := actions_model.GetRunJobsByRunAndAttemptID(t.Context(), runID, attemptID)
	require.NoError(t, err)
	anchors := make(map[string][]string)
	for _, job := range jobs {
		if len(job.DeferredMatrixPayload) > 0 {
			key := fmt.Sprintf("%d/%s", job.ParentJobID, job.JobID)
			anchors[key] = append(anchors[key], job.Name)
		}
	}
	for key, names := range anchors {
		assert.Len(t, names, 1, "job %s must have exactly one anchor row, got %v", key, names)
	}
}

// TestRerunReDerivesDeferredMatrix covers the rerun side of dynamic matrix
func TestRerunReDerivesDeferredMatrix(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	// rerunLatestAttempt reruns jobsToRerun (nil = the whole run) through the real plan/exec pipeline,
	// bypassing only validateRerun, whose repo-unit checks need a full web fixture.
	rerunLatestAttempt := func(t *testing.T, repoID, runID int64, jobsToRerun []*actions_model.ActionRunJob) *actions_model.ActionRunAttempt {
		t.Helper()
		ctx := t.Context()
		defer test.MockVariableValue(&EmitJobsIfReadyByRun, func(int64) error { return nil })()

		run, err := actions_model.GetRunByRepoAndID(ctx, repoID, runID)
		require.NoError(t, err)
		plan, err := buildRerunPlan(ctx, run, &user_model.User{ID: 1}, jobsToRerun)
		require.NoError(t, err)
		attempt, err := execRerunPlan(ctx, plan)
		require.NoError(t, err)
		return attempt
	}

	// attemptJobsByName loads an attempt's jobs indexed by Name.
	attemptJobsByName := func(t *testing.T, runID, attemptID int64) map[string]*actions_model.ActionRunJob {
		t.Helper()
		jobs, err := actions_model.GetRunJobsByRunAndAttemptID(t.Context(), runID, attemptID)
		require.NoError(t, err)
		out := make(map[string]*actions_model.ActionRunJob, len(jobs))
		for _, job := range jobs {
			out[job.Name] = job
		}
		require.Len(t, out, len(jobs), "job names must be unique within the attempt")
		return out
	}

	// expandFirstAttempt expands the placeholder into build (a|b|c) and marks every job successful, leaving the run ready to be rerun.
	expandFirstAttempt := func(t *testing.T, job *actions_model.ActionRunJob) {
		t.Helper()
		siblings, err := expandDeferredMatrix(t.Context(), job, nil)
		require.NoError(t, err)
		require.Len(t, siblings, 2)
		_, err = db.Exec(t.Context(), "UPDATE `action_run_job` SET status = ? WHERE run_id = ?",
			int(actions_model.StatusSuccess), job.RunID)
		require.NoError(t, err)
	}

	setup := func(t *testing.T) (job, generateTpl *actions_model.ActionRunJob) {
		t.Helper()
		job = setupDeferredMatrixJob(t, "${{ fromJson(needs.generate.outputs.values) }}", "", map[string]string{"values": `["a","b","c"]`})
		jobs, err := actions_model.GetRunJobsByRunAndAttemptID(t.Context(), job.RunID, job.RunAttemptID)
		require.NoError(t, err)
		for _, j := range jobs {
			if j.JobID == "generate" {
				generateTpl = j
			}
		}
		require.NotNil(t, generateTpl)
		return job, generateTpl
	}

	t.Run("rerun need collapses the combinations and re-derives from fresh outputs", func(t *testing.T) {
		job, generateTpl := setup(t)
		expandFirstAttempt(t, job)

		attempt2 := rerunLatestAttempt(t, job.RepoID, job.RunID, []*actions_model.ActionRunJob{generateTpl})
		byName := attemptJobsByName(t, job.RunID, attempt2.ID)
		require.Len(t, byName, 3, "caller + generate + one restored placeholder, no combination clones")

		placeholder := byName["build"]
		require.NotNil(t, placeholder, "the combinations must collapse back into the raw placeholder")
		assert.Equal(t, actions_model.StatusBlocked, placeholder.Status)
		assert.True(t, placeholder.IsMatrixDeferred)
		assert.Equal(t, string(placeholder.DeferredMatrixPayload), string(placeholder.WorkflowPayload))
		assert.Equal(t, job.AttemptJobID, placeholder.AttemptJobID, "the placeholder keeps the group's original id")
		assert.Equal(t, actions_model.StatusWaiting, byName["generate"].Status)

		// The need re-runs with different outputs; re-expansion must follow them, not the old combos.
		finishJobWithOutputs(t, byName["generate"], job.RunID+1000000, map[string]string{"values": `["x","y"]`})
		siblings, err := expandDeferredMatrix(t.Context(), placeholder, nil)
		require.NoError(t, err)
		require.Len(t, siblings, 1)
		assert.Equal(t, "build (x)", placeholder.Name)
		assert.Equal(t, "build (y)", siblings[0].Name)
	})

	t.Run("unchanged outputs keep combination ids stable across attempts", func(t *testing.T) {
		job, generateTpl := setup(t)
		expandFirstAttempt(t, job)
		firstIDs := map[string]int64{}
		for name, j := range attemptJobsByName(t, job.RunID, job.RunAttemptID) {
			firstIDs[name] = j.AttemptJobID
		}

		attempt2 := rerunLatestAttempt(t, job.RepoID, job.RunID, []*actions_model.ActionRunJob{generateTpl})
		byName := attemptJobsByName(t, job.RunID, attempt2.ID)
		finishJobWithOutputs(t, byName["generate"], job.RunID+2000000, map[string]string{"values": `["a","b","c"]`})

		placeholder := byName["build"]
		siblings, err := expandDeferredMatrix(t.Context(), placeholder, nil)
		require.NoError(t, err)
		require.Len(t, siblings, 2)
		assert.Equal(t, firstIDs["build (a)"], placeholder.AttemptJobID)
		for _, sibling := range siblings {
			assert.Equal(t, firstIDs[sibling.Name], sibling.AttemptJobID, "prior-attempt ids are matched by name instead of burning fresh ones")
		}
	})

	t.Run("re-running one combination keeps its matrix value", func(t *testing.T) {
		job, _ := setup(t)
		expandFirstAttempt(t, job)
		buildB := attemptJobsByName(t, job.RunID, job.RunAttemptID)["build (b)"]
		require.NotNil(t, buildB)

		attempt2 := rerunLatestAttempt(t, job.RepoID, job.RunID, []*actions_model.ActionRunJob{buildB})
		byName := attemptJobsByName(t, job.RunID, attempt2.ID)
		require.Len(t, byName, 5, "no collapse: every combination row is cloned")

		clone := byName["build (b)"]
		assert.Equal(t, actions_model.StatusWaiting, clone.Status, "a baked combination is dispatchable as-is")
		assert.False(t, clone.IsMatrixDeferred)
		assert.Contains(t, string(clone.WorkflowPayload), "- b", "the payload stays baked with the old matrix value")
		assert.Equal(t, actions_model.StatusSuccess, byName["build (a)"].Status, "other combinations pass through")
		// The anchor is cloned along with the group, so the next rerun can still collapse it.
		assert.NotEmpty(t, byName["build (a)"].DeferredMatrixPayload)
		assertSingleMatrixAnchor(t, job.RunID, attempt2.ID)
	})

	t.Run("a cancelled unexpanded placeholder re-runs through the emitter", func(t *testing.T) {
		job, _ := setup(t)
		_, err := db.Exec(t.Context(), "UPDATE `action_run_job` SET status = ? WHERE id = ?",
			int(actions_model.StatusCancelled), job.ID)
		require.NoError(t, err)
		job.Status = actions_model.StatusCancelled

		attempt2 := rerunLatestAttempt(t, job.RepoID, job.RunID, []*actions_model.ActionRunJob{job})
		placeholder := attemptJobsByName(t, job.RunID, attempt2.ID)["build"]
		require.NotNil(t, placeholder)
		assert.Equal(t, actions_model.StatusBlocked, placeholder.Status, "never Waiting: dispatching the raw payload would hand the runner an unexpanded matrix")
		assert.True(t, placeholder.IsMatrixDeferred)

		// The pass-through need still exposes its outputs, so the emitter can expand now.
		siblings, err := expandDeferredMatrix(t.Context(), placeholder, nil)
		require.NoError(t, err)
		assert.Len(t, siblings, 2)
		assert.Equal(t, "build (a)", placeholder.Name)
	})

	t.Run("a terminally failed expansion re-runs Blocked and fails deterministically again", func(t *testing.T) {
		job := setupDeferredMatrixJob(t, "${{ fromJson(needs.generate.outputs.missing) }}", "", map[string]string{"values": `["a"]`})
		_, err := expandDeferredMatrix(t.Context(), job, nil)
		require.NoError(t, err)
		require.Equal(t, actions_model.StatusFailure, job.Status)

		attempt2 := rerunLatestAttempt(t, job.RepoID, job.RunID, []*actions_model.ActionRunJob{job})
		placeholder := attemptJobsByName(t, job.RunID, attempt2.ID)["build"]
		require.NotNil(t, placeholder)
		assert.Equal(t, actions_model.StatusBlocked, placeholder.Status)
		assert.True(t, placeholder.IsMatrixDeferred)

		_, err = expandDeferredMatrix(t.Context(), placeholder, nil)
		require.NoError(t, err)
		assert.Equal(t, actions_model.StatusFailure, placeholder.Status, "the matrix is still unresolvable, so the job fails again instead of running with a raw payload")
	})
}
