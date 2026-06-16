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
			name: "max-parallel=1 promotes exactly one blocked job when one slot is open",
			jobs: actions_model.ActionJobList{
				{ID: 1, JobID: "build", Status: actions_model.StatusRunning, Needs: []string{}, MaxParallel: 1},
				{ID: 2, JobID: "build", Status: actions_model.StatusBlocked, Needs: []string{}, MaxParallel: 1},
				{ID: 3, JobID: "build", Status: actions_model.StatusBlocked, Needs: []string{}, MaxParallel: 1},
			},
			want: map[int64]actions_model.Status{},
		},
		{
			name: "max-parallel=1 promotes one job after running job finishes",
			jobs: actions_model.ActionJobList{
				{ID: 1, JobID: "build", Status: actions_model.StatusSuccess, Needs: []string{}, MaxParallel: 1},
				{ID: 2, JobID: "build", Status: actions_model.StatusBlocked, Needs: []string{}, MaxParallel: 1},
				{ID: 3, JobID: "build", Status: actions_model.StatusBlocked, Needs: []string{}, MaxParallel: 1},
			},
			want: nil, // map iteration is non-deterministic; checked by count below
		},
		{
			name: "max-parallel=2 does not promote when limit is reached",
			jobs: actions_model.ActionJobList{
				{ID: 1, JobID: "test", Status: actions_model.StatusRunning, Needs: []string{}, MaxParallel: 2},
				{ID: 2, JobID: "test", Status: actions_model.StatusRunning, Needs: []string{}, MaxParallel: 2},
				{ID: 3, JobID: "test", Status: actions_model.StatusBlocked, Needs: []string{}, MaxParallel: 2},
				{ID: 4, JobID: "test", Status: actions_model.StatusBlocked, Needs: []string{}, MaxParallel: 2},
			},
			want: map[int64]actions_model.Status{},
		},
		{
			name: "max-parallel=2 promotes one job when one slot opens",
			jobs: actions_model.ActionJobList{
				{ID: 1, JobID: "test", Status: actions_model.StatusSuccess, Needs: []string{}, MaxParallel: 2},
				{ID: 2, JobID: "test", Status: actions_model.StatusRunning, Needs: []string{}, MaxParallel: 2},
				{ID: 3, JobID: "test", Status: actions_model.StatusBlocked, Needs: []string{}, MaxParallel: 2},
				{ID: 4, JobID: "test", Status: actions_model.StatusBlocked, Needs: []string{}, MaxParallel: 2},
			},
			want: nil, // checked by count below
		},
		// Cancel corner cases: a cancelled job frees its slot just like a successful/failed job.
		{
			name: "max-parallel=1: cancelled running job frees slot for one blocked job",
			jobs: actions_model.ActionJobList{
				{ID: 1, JobID: "build", Status: actions_model.StatusCancelled, Needs: []string{}, MaxParallel: 1},
				{ID: 2, JobID: "build", Status: actions_model.StatusBlocked, Needs: []string{}, MaxParallel: 1},
				{ID: 3, JobID: "build", Status: actions_model.StatusBlocked, Needs: []string{}, MaxParallel: 1},
			},
			want: nil, // exactly 1 promoted; checked by count below
		},
		{
			name: "max-parallel=1: cancelled waiting job frees slot for one blocked job",
			jobs: actions_model.ActionJobList{
				{ID: 1, JobID: "build", Status: actions_model.StatusRunning, Needs: []string{}, MaxParallel: 2},
				{ID: 2, JobID: "build", Status: actions_model.StatusCancelled, Needs: []string{}, MaxParallel: 2},
				{ID: 3, JobID: "build", Status: actions_model.StatusBlocked, Needs: []string{}, MaxParallel: 2},
				{ID: 4, JobID: "build", Status: actions_model.StatusBlocked, Needs: []string{}, MaxParallel: 2},
			},
			want: nil, // exactly 1 promoted (running=1 + new waiting=1 == max-parallel=2)
		},
		{
			name: "max-parallel=2: two cancelled jobs free two slots for two blocked jobs",
			jobs: actions_model.ActionJobList{
				{ID: 1, JobID: "test", Status: actions_model.StatusCancelled, Needs: []string{}, MaxParallel: 2},
				{ID: 2, JobID: "test", Status: actions_model.StatusCancelled, Needs: []string{}, MaxParallel: 2},
				{ID: 3, JobID: "test", Status: actions_model.StatusBlocked, Needs: []string{}, MaxParallel: 2},
				{ID: 4, JobID: "test", Status: actions_model.StatusBlocked, Needs: []string{}, MaxParallel: 2},
			},
			// Both slots are free, so both blocked jobs should be promoted.
			want: map[int64]actions_model.Status{
				3: actions_model.StatusWaiting,
				4: actions_model.StatusWaiting,
			},
		},
		{
			name: "max-parallel=2: one running, one cancelled – only one slot free",
			jobs: actions_model.ActionJobList{
				{ID: 1, JobID: "test", Status: actions_model.StatusRunning, Needs: []string{}, MaxParallel: 2},
				{ID: 2, JobID: "test", Status: actions_model.StatusCancelled, Needs: []string{}, MaxParallel: 2},
				{ID: 3, JobID: "test", Status: actions_model.StatusBlocked, Needs: []string{}, MaxParallel: 2},
				{ID: 4, JobID: "test", Status: actions_model.StatusBlocked, Needs: []string{}, MaxParallel: 2},
			},
			want: nil, // exactly 1 promoted (running=1 + new waiting=1 == max-parallel=2)
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
			got := r.Resolve(ctx)
			if tt.want == nil {
				waitingCount := 0
				for _, s := range got {
					if s == actions_model.StatusWaiting {
						waitingCount++
					}
				}
				assert.Equal(t, 1, waitingCount, "expected exactly 1 job promoted to Waiting, got %v", got)
			} else {
				assert.Equal(t, want, got)
			}
		})
	}
}

func Test_maxParallelWorkflowLifecycle(t *testing.T) {
	const matrixJobID = "matrix"

	countStatus := func(jobs actions_model.ActionJobList, s actions_model.Status) int {
		n := 0
		for _, j := range jobs {
			if j.Status == s {
				n++
			}
		}
		return n
	}

	applyUpdates := func(jobs actions_model.ActionJobList, updates map[int64]actions_model.Status) {
		for _, j := range jobs {
			if s, ok := updates[j.ID]; ok {
				j.Status = s
			}
		}
	}

	// pickUpAll simulates every WAITING job being accepted by a runner.
	pickUpAll := func(jobs actions_model.ActionJobList) {
		for _, j := range jobs {
			if j.Status == actions_model.StatusWaiting {
				j.Status = actions_model.StatusRunning
			}
		}
	}

	// completeOne marks the first RUNNING job as SUCCESS.
	completeOne := func(jobs actions_model.ActionJobList) {
		for _, j := range jobs {
			if j.Status == actions_model.StatusRunning {
				j.Status = actions_model.StatusSuccess
				return
			}
		}
	}

	makeJobs := func(n, maxParallel int) actions_model.ActionJobList {
		list := make(actions_model.ActionJobList, n)
		for i := range n {
			list[i] = &actions_model.ActionRunJob{
				ID:          int64(i + 1),
				JobID:       matrixJobID,
				Status:      actions_model.StatusBlocked,
				Needs:       []string{},
				MaxParallel: maxParallel,
			}
		}
		return list
	}

	runResolve := func(t *testing.T, jobs actions_model.ActionJobList) map[int64]actions_model.Status {
		t.Helper()
		return newJobStatusResolver(jobs, nil).Resolve(t.Context())
	}

	// assertSlotInvariant verifies both slot constraints after every resolve cycle.
	// It is a no-op when maxParallel=0 (unlimited).
	// Cancelled jobs count as done: they free their slot just like successful jobs.
	assertSlotInvariant := func(t *testing.T, jobs actions_model.ActionJobList, maxParallel int, label string) {
		t.Helper()
		if maxParallel == 0 {
			return
		}
		running := countStatus(jobs, actions_model.StatusRunning)
		waiting := countStatus(jobs, actions_model.StatusWaiting)
		done := 0
		for _, j := range jobs {
			if j.Status.IsDone() {
				done++
			}
		}
		remaining := len(jobs) - done
		active := running + waiting

		assert.LessOrEqual(t, active, maxParallel,
			"%s: running(%d)+waiting(%d) must not exceed max-parallel(%d)",
			label, running, waiting, maxParallel)

		assert.Equal(t, min(remaining, maxParallel), active,
			"%s: running(%d)+waiting(%d) should equal min(remaining=%d, maxParallel=%d)",
			label, running, waiting, remaining, maxParallel)
	}

	tests := []struct {
		name               string
		totalJobs          int
		maxParallel        int
		wantInitialWaiting int // expected WAITING count after the very first Resolve()
	}{
		{
			// 0 means unlimited: the max-parallel branch in resolve() is skipped
			name:               "max-parallel=0 (unlimited): all 5 jobs start immediately",
			totalJobs:          5,
			maxParallel:        0,
			wantInitialWaiting: 5,
		},
		{
			// Strictest case: one slot, so the resolver must promote exactly 1 job
			name:               "max-parallel=1 (strict serial): exactly 1 job at a time",
			totalJobs:          5,
			maxParallel:        1,
			wantInitialWaiting: 1,
		},
		{
			// Limit higher than job count: behaves like unlimited for this run.
			name:               "max-parallel=10 (N<limit): all 5 jobs start immediately",
			totalJobs:          5,
			maxParallel:        10,
			wantInitialWaiting: 5,
		},
		{
			// Limit lower than job count: first 10 start, remaining 2 stay blocked until slots open up.
			name:               "max-parallel=10 (N>limit): first 10 of 12 start, rest queue",
			totalJobs:          12,
			maxParallel:        10,
			wantInitialWaiting: 10,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			jobs := makeJobs(tc.totalJobs, tc.maxParallel)

			applyUpdates(jobs, runResolve(t, jobs))

			assert.Equal(t, tc.wantInitialWaiting, countStatus(jobs, actions_model.StatusWaiting),
				"phase 1: Resolve should promote exactly %d jobs to WAITING", tc.wantInitialWaiting)
			assert.Equal(t, tc.totalJobs-tc.wantInitialWaiting, countStatus(jobs, actions_model.StatusBlocked),
				"phase 1: remaining %d jobs should still be BLOCKED", tc.totalJobs-tc.wantInitialWaiting)

			pickUpAll(jobs)
			assertSlotInvariant(t, jobs, tc.maxParallel, "phase 2 (after initial pickup)")

			for cycle := 1; cycle <= tc.totalJobs; cycle++ {
				if countStatus(jobs, actions_model.StatusRunning) == 0 {
					break
				}

				completeOne(jobs)
				applyUpdates(jobs, runResolve(t, jobs))

				label := fmt.Sprintf("phase 3 cycle %d", cycle)
				assertSlotInvariant(t, jobs, tc.maxParallel, label)

				pickUpAll(jobs)
			}

			for countStatus(jobs, actions_model.StatusRunning) > 0 {
				completeOne(jobs)
				applyUpdates(jobs, runResolve(t, jobs))
				pickUpAll(jobs)
			}

			assert.Equal(t, tc.totalJobs, countStatus(jobs, actions_model.StatusSuccess),
				"phase 5: all %d jobs must reach SUCCESS", tc.totalJobs)
			assert.Equal(t, 0, countStatus(jobs, actions_model.StatusBlocked),
				"phase 5: no jobs may remain BLOCKED")
			assert.Equal(t, 0, countStatus(jobs, actions_model.StatusWaiting),
				"phase 5: no jobs may remain WAITING")
			assert.Equal(t, 0, countStatus(jobs, actions_model.StatusRunning),
				"phase 5: no jobs may remain RUNNING")
		})
	}
}

// Test_maxParallel_CancelCornerCase verifies that a cancelled job frees its
// max-parallel slot so that a blocked job is promoted to Waiting, matching
// the behaviour of a normally-completed job.
func Test_maxParallel_CancelCornerCase(t *testing.T) {
	const jobID = "matrix"

	countStatus := func(jobs actions_model.ActionJobList, s actions_model.Status) int {
		n := 0
		for _, j := range jobs {
			if j.Status == s {
				n++
			}
		}
		return n
	}

	applyUpdates := func(jobs actions_model.ActionJobList, updates map[int64]actions_model.Status) {
		for _, j := range jobs {
			if s, ok := updates[j.ID]; ok {
				j.Status = s
			}
		}
	}

	pickUpAll := func(jobs actions_model.ActionJobList) {
		for _, j := range jobs {
			if j.Status == actions_model.StatusWaiting {
				j.Status = actions_model.StatusRunning
			}
		}
	}

	// cancelOne marks the first RUNNING job as Cancelled, simulating a runner
	// reporting RESULT_CANCELLED or an admin cancelling via the UI.
	cancelOne := func(jobs actions_model.ActionJobList) {
		for _, j := range jobs {
			if j.Status == actions_model.StatusRunning {
				j.Status = actions_model.StatusCancelled
				return
			}
		}
	}

	// cancelWaiting marks the first WAITING job as Cancelled, simulating an
	// admin cancelling a queued-but-not-yet-picked-up job.
	cancelWaiting := func(jobs actions_model.ActionJobList) {
		for _, j := range jobs {
			if j.Status == actions_model.StatusWaiting {
				j.Status = actions_model.StatusCancelled
				return
			}
		}
	}

	runResolve := func(t *testing.T, jobs actions_model.ActionJobList) map[int64]actions_model.Status {
		t.Helper()
		return newJobStatusResolver(jobs, nil).Resolve(t.Context())
	}

	assertActive := func(t *testing.T, jobs actions_model.ActionJobList, maxParallel int, label string) {
		t.Helper()
		running := countStatus(jobs, actions_model.StatusRunning)
		waiting := countStatus(jobs, actions_model.StatusWaiting)
		done := 0
		for _, j := range jobs {
			if j.Status.IsDone() {
				done++
			}
		}
		remaining := len(jobs) - done
		active := running + waiting
		assert.LessOrEqual(t, active, maxParallel,
			"%s: active(%d) must not exceed max-parallel(%d)", label, active, maxParallel)
		assert.Equal(t, min(remaining, maxParallel), active,
			"%s: active(%d) should equal min(remaining=%d, max-parallel=%d)", label, active, remaining, maxParallel)
	}

	t.Run("cancelled running job frees slot (max-parallel=1)", func(t *testing.T) {
		// 4 matrix jobs, max-parallel=1: only one runs at a time.
		jobs := make(actions_model.ActionJobList, 4)
		for i := range jobs {
			jobs[i] = &actions_model.ActionRunJob{
				ID: int64(i + 1), JobID: jobID,
				Status: actions_model.StatusBlocked, Needs: []string{}, MaxParallel: 1,
			}
		}

		// Phase 1: initial resolve promotes exactly 1 job.
		applyUpdates(jobs, runResolve(t, jobs))
		assert.Equal(t, 1, countStatus(jobs, actions_model.StatusWaiting))
		assert.Equal(t, 3, countStatus(jobs, actions_model.StatusBlocked))

		// Phase 2: runner picks up the waiting job.
		pickUpAll(jobs)
		assert.Equal(t, 1, countStatus(jobs, actions_model.StatusRunning))

		// Phase 3 (corner case): the running job is cancelled.
		// EmitJobsIfReadyByJobs is triggered, which calls checkJobsOfRun → resolve().
		cancelOne(jobs)
		assert.Equal(t, 1, countStatus(jobs, actions_model.StatusCancelled))

		applyUpdates(jobs, runResolve(t, jobs))
		assertActive(t, jobs, 1, "after cancel + resolve")

		// Phase 4: pick up and complete all remaining jobs normally.
		pickUpAll(jobs)
		for countStatus(jobs, actions_model.StatusRunning) > 0 {
			for _, j := range jobs {
				if j.Status == actions_model.StatusRunning {
					j.Status = actions_model.StatusSuccess
				}
			}
			applyUpdates(jobs, runResolve(t, jobs))
			pickUpAll(jobs)
		}

		assert.Equal(t, 1, countStatus(jobs, actions_model.StatusCancelled))
		assert.Equal(t, 3, countStatus(jobs, actions_model.StatusSuccess))
		assert.Equal(t, 0, countStatus(jobs, actions_model.StatusBlocked))
		assert.Equal(t, 0, countStatus(jobs, actions_model.StatusWaiting))
	})

	t.Run("cancelled waiting job frees slot (max-parallel=2)", func(t *testing.T) {
		// 5 matrix jobs, max-parallel=2.
		jobs := make(actions_model.ActionJobList, 5)
		for i := range jobs {
			jobs[i] = &actions_model.ActionRunJob{
				ID: int64(i + 1), JobID: jobID,
				Status: actions_model.StatusBlocked, Needs: []string{}, MaxParallel: 2,
			}
		}

		// Phase 1: resolve promotes 2 jobs.
		applyUpdates(jobs, runResolve(t, jobs))
		assert.Equal(t, 2, countStatus(jobs, actions_model.StatusWaiting))

		// Phase 2: one runner picks up one of the two waiting jobs; the other stays waiting.
		// Use the first actually-Waiting job to avoid non-determinism from map iteration.
		for _, j := range jobs {
			if j.Status == actions_model.StatusWaiting {
				j.Status = actions_model.StatusRunning
				break
			}
		}
		assert.Equal(t, 1, countStatus(jobs, actions_model.StatusRunning))
		assert.Equal(t, 1, countStatus(jobs, actions_model.StatusWaiting))

		// Phase 3 (corner case): admin cancels the still-waiting job before any runner picks it.
		cancelWaiting(jobs)
		assert.Equal(t, 1, countStatus(jobs, actions_model.StatusCancelled))

		// resolve() must refill the freed slot from the blocked queue.
		applyUpdates(jobs, runResolve(t, jobs))
		assertActive(t, jobs, 2, "after waiting-job cancel + resolve")

		// Phase 4: run to completion.
		pickUpAll(jobs)
		for countStatus(jobs, actions_model.StatusRunning) > 0 {
			for _, j := range jobs {
				if j.Status == actions_model.StatusRunning {
					j.Status = actions_model.StatusSuccess
				}
			}
			applyUpdates(jobs, runResolve(t, jobs))
			pickUpAll(jobs)
		}

		assert.Equal(t, 1, countStatus(jobs, actions_model.StatusCancelled))
		assert.Equal(t, 4, countStatus(jobs, actions_model.StatusSuccess))
		assert.Equal(t, 0, countStatus(jobs, actions_model.StatusBlocked))
		assert.Equal(t, 0, countStatus(jobs, actions_model.StatusWaiting))
	})

	t.Run("multiple cancels stay within max-parallel (max-parallel=2)", func(t *testing.T) {
		// 6 jobs, max-parallel=2. Cancel two running jobs back-to-back and verify
		// that each cancel correctly replenishes one slot from the blocked queue.
		jobs := make(actions_model.ActionJobList, 6)
		for i := range jobs {
			jobs[i] = &actions_model.ActionRunJob{
				ID: int64(i + 1), JobID: jobID,
				Status: actions_model.StatusBlocked, Needs: []string{}, MaxParallel: 2,
			}
		}

		applyUpdates(jobs, runResolve(t, jobs))
		assert.Equal(t, 2, countStatus(jobs, actions_model.StatusWaiting))

		pickUpAll(jobs)
		assert.Equal(t, 2, countStatus(jobs, actions_model.StatusRunning))

		// Cancel first running job → one blocked job should be promoted.
		cancelOne(jobs)
		applyUpdates(jobs, runResolve(t, jobs))
		assertActive(t, jobs, 2, "after first cancel")
		pickUpAll(jobs)

		// Cancel second running job → another blocked job should be promoted.
		cancelOne(jobs)
		applyUpdates(jobs, runResolve(t, jobs))
		assertActive(t, jobs, 2, "after second cancel")
		pickUpAll(jobs)

		// Complete all remaining running jobs.
		for countStatus(jobs, actions_model.StatusRunning) > 0 {
			for _, j := range jobs {
				if j.Status == actions_model.StatusRunning {
					j.Status = actions_model.StatusSuccess
				}
			}
			applyUpdates(jobs, runResolve(t, jobs))
			pickUpAll(jobs)
		}

		assert.Equal(t, 2, countStatus(jobs, actions_model.StatusCancelled))
		assert.Equal(t, 4, countStatus(jobs, actions_model.StatusSuccess))
		assert.Equal(t, 0, countStatus(jobs, actions_model.StatusBlocked))
		assert.Equal(t, 0, countStatus(jobs, actions_model.StatusWaiting))
	})
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
	result, err := checkRunConcurrency(ctx, runA)
	assert.NoError(t, err)

	if assert.Len(t, result.Jobs, 1) {
		assert.Equal(t, jobBBlocked.ID, result.Jobs[0].ID)
	}
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
