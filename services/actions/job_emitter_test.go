// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"fmt"
	"testing"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

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
		{
			name: "`if` is empty and a failed need has continue-on-error",
			jobs: actions_model.ActionJobList{
				{ID: 1, JobID: "job1", Status: actions_model.StatusFailure, ContinueOnError: true, Needs: []string{}},
				{ID: 2, JobID: "job2", Status: actions_model.StatusBlocked, Needs: []string{"job1"}, WorkflowPayload: []byte(
					`
name: test
on: push
jobs:
  job2:
    runs-on: ubuntu-latest
    needs: job1
    steps:
      - run: echo "should run, job1 failure is masked by continue-on-error"
`)},
			},
			want: map[int64]actions_model.Status{2: actions_model.StatusWaiting},
		},
	}
	assert.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	stubRun := &actions_model.ActionRun{TriggerUser: &user_model.User{}, Repo: &repo_model.Repository{}}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Each subtest gets a unique RunID / RunAttemptID so jobs from different subtests don't bleed into each other's FindTaskNeeds queries
			runID := int64(9001 + i)
			attemptID := int64(9001 + i)

			// Insert each test job (letting the DB assign IDs) and remember the testID -> dbID mapping so we can translate the expected map.
			idMap := make(map[int64]int64, len(tt.jobs))
			for _, j := range tt.jobs {
				origID := j.ID
				j.ID = 0
				j.RunID = runID
				j.RunAttemptID = attemptID
				j.Run = stubRun

				// The resolver evaluates Blocked jobs via evaluateJobIf, which needs a valid YAML payload;
				// supply a minimal one when the case didn't.
				if j.Status == actions_model.StatusBlocked && len(j.WorkflowPayload) == 0 {
					j.WorkflowPayload = fmt.Appendf(nil, `name: test
on: push
jobs:
  %s:
    runs-on: ubuntu-latest
    steps:
      - run: echo
`, j.JobID)
				}

				assert.NoError(t, db.Insert(ctx, j))
				idMap[origID] = j.ID
			}

			want := make(map[int64]actions_model.Status, len(tt.want))
			for k, v := range tt.want {
				want[idMap[k]] = v
			}

			r := newJobStatusResolver(tt.jobs, nil)
			assert.Equal(t, want, r.Resolve(ctx))
		})
	}
}

// Test_checkRunConcurrency_NoDuplicateConcurrencyGroupCheck verifies that when a run's
// ConcurrencyGroup has already been checked at the run level, the same group is not
// re-checked for individual jobs.
func Test_checkRunConcurrency_NoDuplicateConcurrencyGroupCheck(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	// Run A: the triggering run of attempt A. It is done, so it no longer holds "test-cg", which is what lets checkRunConcurrency wake the blocked waiter.
	runA := &actions_model.ActionRun{
		RepoID:        4,
		OwnerID:       1,
		TriggerUserID: 1,
		WorkflowID:    "test.yml",
		Index:         9901,
		Ref:           "refs/heads/main",
		Status:        actions_model.StatusSuccess,
	}
	assert.NoError(t, db.Insert(ctx, runA))

	// Attempt A: a done attempt of run A with concurrency group "test-cg"
	runAAttempt := &actions_model.ActionRunAttempt{
		RepoID:           4,
		RunID:            runA.ID,
		Attempt:          1,
		Status:           actions_model.StatusSuccess,
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
	result, err := checkRunConcurrency(ctx, runA)
	assert.NoError(t, err)

	// "test-cg" is free, so the single blocked waiter (run B) is collected for re-emit.
	if assert.Len(t, result.RunIDsToReEmit, 1) {
		assert.Equal(t, runB.ID, result.RunIDsToReEmit[0])
	}
	assert.Empty(t, result.Jobs)
}

// Test_checkJobsOfCurrentRunAttempt_RunLevelConcurrencyKeepsJobsBlocked verifies that
// the resolver does not transition a job out of Blocked while another run still holds
// the workflow-level concurrency group. Regression for #37446.
func Test_checkJobsOfCurrentRunAttempt_RunLevelConcurrencyKeepsJobsBlocked(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	const group = "test-run-level-concurrency-keeps-blocked"

	// Holder run: Running attempt in the concurrency group.
	holderRun := &actions_model.ActionRun{
		RepoID: 4, OwnerID: 1, TriggerUserID: 1,
		WorkflowID: "test.yml", Index: 9911, Ref: "refs/heads/main",
		Status: actions_model.StatusRunning,
	}
	assert.NoError(t, db.Insert(ctx, holderRun))
	holderAttempt := &actions_model.ActionRunAttempt{
		RepoID: 4, RunID: holderRun.ID, Attempt: 1,
		Status: actions_model.StatusRunning, ConcurrencyGroup: group,
	}
	assert.NoError(t, db.Insert(ctx, holderAttempt))
	_, err := db.Exec(ctx, "UPDATE `action_run` SET latest_attempt_id = ? WHERE id = ?", holderAttempt.ID, holderRun.ID)
	assert.NoError(t, err)

	// Blocked run: Blocked attempt in the same group, with one Blocked job that has
	// no needs and no job-level concurrency. Without the run-level guard in
	// checkJobsOfCurrentRunAttempt, the resolver would transition this job to Waiting.
	blockedRun := &actions_model.ActionRun{
		RepoID: 4, OwnerID: 1, TriggerUserID: 1,
		WorkflowID: "test.yml", Index: 9912, Ref: "refs/heads/main",
		Status: actions_model.StatusBlocked,
	}
	assert.NoError(t, db.Insert(ctx, blockedRun))
	blockedAttempt := &actions_model.ActionRunAttempt{
		RepoID: 4, RunID: blockedRun.ID, Attempt: 1,
		Status: actions_model.StatusBlocked, ConcurrencyGroup: group,
	}
	assert.NoError(t, db.Insert(ctx, blockedAttempt))
	_, err = db.Exec(ctx, "UPDATE `action_run` SET latest_attempt_id = ? WHERE id = ?", blockedAttempt.ID, blockedRun.ID)
	assert.NoError(t, err)
	blockedRun.LatestAttemptID = blockedAttempt.ID
	blockedJob := &actions_model.ActionRunJob{
		RunID: blockedRun.ID, RunAttemptID: blockedAttempt.ID, AttemptJobID: 1,
		RepoID: 4, OwnerID: 1, JobID: "job1", Name: "job1",
		Status: actions_model.StatusBlocked,
		WorkflowPayload: []byte(`
name: test
on: push
jobs:
  job1:
    runs-on: ubuntu-latest
    steps:
      - run: echo
`),
	}
	assert.NoError(t, db.Insert(ctx, blockedJob))

	result, err := checkJobsOfCurrentRunAttempt(ctx, blockedRun)
	assert.NoError(t, err)
	assert.Empty(t, result.UpdatedJobs)

	refreshed := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: blockedJob.ID})
	assert.Equal(t, actions_model.StatusBlocked, refreshed.Status)
}

// Test_checkRunConcurrency_HeldGroupDoesNotWake verifies that only an unoccupied concurrency group can wake up a blocked run/job.
func Test_checkRunConcurrency_HeldGroupDoesNotWake(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	// Run A holds "test-cg": its attempt is still running.
	runA := &actions_model.ActionRun{
		RepoID: 4, OwnerID: 1, TriggerUserID: 1, WorkflowID: "test.yml",
		Index: 9911, Ref: "refs/heads/main", Status: actions_model.StatusRunning,
	}
	assert.NoError(t, db.Insert(ctx, runA))
	runAAttempt := &actions_model.ActionRunAttempt{
		RepoID: 4, RunID: runA.ID, Attempt: 1, Status: actions_model.StatusRunning, ConcurrencyGroup: "test-cg",
	}
	assert.NoError(t, db.Insert(ctx, runAAttempt))
	_, err := db.Exec(ctx, "UPDATE `action_run` SET latest_attempt_id = ? WHERE id = ?", runAAttempt.ID, runA.ID)
	assert.NoError(t, err)

	// Run B is blocked on the same group.
	runB := &actions_model.ActionRun{
		RepoID: 4, OwnerID: 1, TriggerUserID: 1, WorkflowID: "test.yml",
		Index: 9912, Ref: "refs/heads/main", Status: actions_model.StatusBlocked,
	}
	assert.NoError(t, db.Insert(ctx, runB))
	runBAttempt := &actions_model.ActionRunAttempt{
		RepoID: 4, RunID: runB.ID, Attempt: 1, Status: actions_model.StatusBlocked, ConcurrencyGroup: "test-cg",
	}
	assert.NoError(t, db.Insert(ctx, runBAttempt))
	_, err = db.Exec(ctx, "UPDATE `action_run` SET latest_attempt_id = ? WHERE id = ?", runBAttempt.ID, runB.ID)
	assert.NoError(t, err)

	runA, _, _ = db.GetByID[actions_model.ActionRun](ctx, runA.ID)
	result, err := checkRunConcurrency(ctx, runA)
	assert.NoError(t, err)

	// The group is held by run A, so run B must not be woken; A will wake it when it releases the group.
	assert.Empty(t, result.RunIDsToReEmit)
}

// Test_findConcurrencyWaiterToWake covers the finder's contract: it skips the run being processed (excludeRunID),
// returns another blocked waiter when the group is free, and returns 0 while the group is still held.
func Test_findConcurrencyWaiterToWake(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	const repoID int64 = 4
	seed := func(index int64, group string, status actions_model.Status) *actions_model.ActionRun {
		run := &actions_model.ActionRun{
			RepoID: repoID, OwnerID: 1, TriggerUserID: 1, WorkflowID: "test.yml",
			Index: index, Ref: "refs/heads/main", Status: status,
		}
		assert.NoError(t, db.Insert(ctx, run))
		assert.NoError(t, db.Insert(ctx, &actions_model.ActionRunAttempt{
			RepoID: repoID, RunID: run.ID, Attempt: 1, Status: status, ConcurrencyGroup: group,
		}))
		return run
	}

	// Free group "excl-cg" with two blocked runs: excluding self returns the other waiter, not self.
	self := seed(99701, "excl-cg", actions_model.StatusBlocked)
	other := seed(99702, "excl-cg", actions_model.StatusBlocked)
	id, err := findConcurrencyWaiterToWake(ctx, repoID, self.ID, "excl-cg")
	assert.NoError(t, err)
	assert.Equal(t, other.ID, id)

	// Free group "solo-cg" with only self blocked: excluding it leaves no waiter.
	solo := seed(99703, "solo-cg", actions_model.StatusBlocked)
	id, err = findConcurrencyWaiterToWake(ctx, repoID, solo.ID, "solo-cg")
	assert.NoError(t, err)
	assert.Equal(t, int64(0), id)

	// Held group "held-cg" (a running holder) has a blocked waiter, but nothing is woken while held.
	seed(99704, "held-cg", actions_model.StatusRunning)
	seed(99705, "held-cg", actions_model.StatusBlocked)
	id, err = findConcurrencyWaiterToWake(ctx, repoID, 0, "held-cg")
	assert.NoError(t, err)
	assert.Equal(t, int64(0), id)
}
