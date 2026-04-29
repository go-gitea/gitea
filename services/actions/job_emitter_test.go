// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func Test_jobStatusResolver_Resolve(t *testing.T) {
	tests := []struct {
		name string
		jobs actions_model.ActionJobList
		want map[int64]actions_model.Status
	}{
		{
			name: "no blocked",
			jobs: actions_model.ActionJobList{
				{ID: 1, JobID: "1", Status: actions_model.StatusWaiting, Needs: []string{}},
				{ID: 2, JobID: "2", Status: actions_model.StatusWaiting, Needs: []string{}},
				{ID: 3, JobID: "3", Status: actions_model.StatusWaiting, Needs: []string{}},
			},
			want: map[int64]actions_model.Status{},
		},
		{
			name: "single blocked",
			jobs: actions_model.ActionJobList{
				{ID: 1, JobID: "1", Status: actions_model.StatusSuccess, Needs: []string{}},
				{ID: 2, JobID: "2", Status: actions_model.StatusBlocked, Needs: []string{"1"}},
				{ID: 3, JobID: "3", Status: actions_model.StatusWaiting, Needs: []string{}},
			},
			want: map[int64]actions_model.Status{
				2: actions_model.StatusWaiting,
			},
		},
		{
			name: "multiple blocked",
			jobs: actions_model.ActionJobList{
				{ID: 1, JobID: "1", Status: actions_model.StatusSuccess, Needs: []string{}},
				{ID: 2, JobID: "2", Status: actions_model.StatusBlocked, Needs: []string{"1"}},
				{ID: 3, JobID: "3", Status: actions_model.StatusBlocked, Needs: []string{"1"}},
			},
			want: map[int64]actions_model.Status{
				2: actions_model.StatusWaiting,
				3: actions_model.StatusWaiting,
			},
		},
		{
			name: "chain blocked",
			jobs: actions_model.ActionJobList{
				{ID: 1, JobID: "1", Status: actions_model.StatusFailure, Needs: []string{}},
				{ID: 2, JobID: "2", Status: actions_model.StatusBlocked, Needs: []string{"1"}},
				{ID: 3, JobID: "3", Status: actions_model.StatusBlocked, Needs: []string{"2"}},
			},
			want: map[int64]actions_model.Status{
				2: actions_model.StatusSkipped,
				3: actions_model.StatusSkipped,
			},
		},
		{
			name: "loop need",
			jobs: actions_model.ActionJobList{
				{ID: 1, JobID: "1", Status: actions_model.StatusBlocked, Needs: []string{"3"}},
				{ID: 2, JobID: "2", Status: actions_model.StatusBlocked, Needs: []string{"1"}},
				{ID: 3, JobID: "3", Status: actions_model.StatusBlocked, Needs: []string{"2"}},
			},
			want: map[int64]actions_model.Status{},
		},
		{
			name: "`if` is not empty and all jobs in `needs` completed successfully",
			jobs: actions_model.ActionJobList{
				{ID: 1, JobID: "job1", Status: actions_model.StatusSuccess, Needs: []string{}},
				{ID: 2, JobID: "job2", Status: actions_model.StatusBlocked, Needs: []string{"job1"}, WorkflowPayload: []byte(
					`
name: test
on: push
jobs:
  job2:
    runs-on: ubuntu-latest
    needs: job1
    if: ${{ always() && needs.job1.result == 'success' }}
    steps:
      - run: echo "will be checked by act_runner"
`)},
			},
			want: map[int64]actions_model.Status{2: actions_model.StatusWaiting},
		},
		{
			name: "`if` is not empty and not all jobs in `needs` completed successfully",
			jobs: actions_model.ActionJobList{
				{ID: 1, JobID: "job1", Status: actions_model.StatusFailure, Needs: []string{}},
				{ID: 2, JobID: "job2", Status: actions_model.StatusBlocked, Needs: []string{"job1"}, WorkflowPayload: []byte(
					`
name: test
on: push
jobs:
  job2:
    runs-on: ubuntu-latest
    needs: job1
    if: ${{ always() && needs.job1.result == 'failure' }}
    steps:
      - run: echo "will be checked by act_runner"
`)},
			},
			want: map[int64]actions_model.Status{2: actions_model.StatusWaiting},
		},
		{
			name: "`if` is empty and not all jobs in `needs` completed successfully",
			jobs: actions_model.ActionJobList{
				{ID: 1, JobID: "job1", Status: actions_model.StatusFailure, Needs: []string{}},
				{ID: 2, JobID: "job2", Status: actions_model.StatusBlocked, Needs: []string{"job1"}, WorkflowPayload: []byte(
					`
name: test
on: push
jobs:
  job2:
    runs-on: ubuntu-latest
    needs: job1
    steps:
      - run: echo "should be skipped"
`)},
			},
			want: map[int64]actions_model.Status{2: actions_model.StatusSkipped},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newJobStatusResolver(tt.jobs, nil)
			assert.Equal(t, tt.want, r.Resolve(t.Context()))
		})
	}
}

// Test_checkRunConcurrency_NoDuplicateConcurrencyGroupCheck verifies that when a run's
// ConcurrencyGroup has already been checked at the run level, the same group is not
// re-checked for individual jobs.
func Test_checkRunConcurrency_NoDuplicateConcurrencyGroupCheck(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	// Run A: the triggering run of attempt A
	runA := &actions_model.ActionRun{
		RepoID:        4,
		OwnerID:       1,
		TriggerUserID: 1,
		WorkflowID:    "test.yml",
		Index:         9901,
		Ref:           "refs/heads/main",
		Status:        actions_model.StatusRunning,
	}
	assert.NoError(t, db.Insert(ctx, runA))

	// Attempt A: an attempt of run A with concurrency group "test-cg"
	runAAttempt := &actions_model.ActionRunAttempt{
		RepoID:           4,
		RunID:            runA.ID,
		Attempt:          1,
		Status:           actions_model.StatusRunning,
		ConcurrencyGroup: "test-cg",
	}
	assert.NoError(t, db.Insert(ctx, runAAttempt))
	_, err := db.Exec(t.Context(), "UPDATE `action_run` SET latest_attempt_id = ? WHERE id = ?", runAAttempt.ID, runA.ID)
	assert.NoError(t, err)

	// A done job for run A with the same ConcurrencyGroup.
	// This triggers the job-level concurrency check in checkRunConcurrency.
	jobADone := &actions_model.ActionRunJob{
		RunID:            runA.ID,
		RunAttemptID:     runAAttempt.ID,
		AttemptJobID:     1,
		RepoID:           4,
		OwnerID:          1,
		JobID:            "job1",
		Name:             "job1",
		Status:           actions_model.StatusSuccess,
		ConcurrencyGroup: "test-cg",
	}
	assert.NoError(t, db.Insert(ctx, jobADone))

	// Run B: a run blocked by concurrency
	runB := &actions_model.ActionRun{
		RepoID:        4,
		OwnerID:       1,
		TriggerUserID: 1,
		WorkflowID:    "test.yml",
		Index:         9902,
		Ref:           "refs/heads/main",
		Status:        actions_model.StatusBlocked,
	}
	assert.NoError(t, db.Insert(ctx, runB))

	// Attempt B: an blocked attempt of run B
	runBAttempt := &actions_model.ActionRunAttempt{
		RepoID:           4,
		RunID:            runB.ID,
		Attempt:          1,
		Status:           actions_model.StatusBlocked,
		ConcurrencyGroup: "test-cg",
	}
	assert.NoError(t, db.Insert(ctx, runBAttempt))
	_, err = db.Exec(t.Context(), "UPDATE `action_run` SET latest_attempt_id = ? WHERE id = ?", runBAttempt.ID, runB.ID)
	assert.NoError(t, err)

	// A blocked job belonging to run B (no job-level concurrency group).
	jobBBlocked := &actions_model.ActionRunJob{
		RunID:        runB.ID,
		RunAttemptID: runBAttempt.ID,
		AttemptJobID: 1,
		RepoID:       4,
		OwnerID:      1,
		JobID:        "job1",
		Name:         "job1",
		Status:       actions_model.StatusBlocked,
	}
	assert.NoError(t, db.Insert(ctx, jobBBlocked))

	runA, _, _ = db.GetByID[actions_model.ActionRun](t.Context(), runA.ID)
	jobs, _, _, err := checkRunConcurrency(ctx, runA)
	assert.NoError(t, err)

	if assert.Len(t, jobs, 1) {
		assert.Equal(t, jobBBlocked.ID, jobs[0].ID)
	}
}
