// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"fmt"
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"

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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newJobStatusResolver(tt.jobs, nil)
			got := r.Resolve(t.Context())
			if tt.want == nil {
				waitingCount := 0
				for _, s := range got {
					if s == actions_model.StatusWaiting {
						waitingCount++
					}
				}
				assert.Equal(t, 1, waitingCount, "expected exactly 1 job promoted to Waiting, got %v", got)
			} else {
				assert.Equal(t, tt.want, got)
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
	assertSlotInvariant := func(t *testing.T, jobs actions_model.ActionJobList, maxParallel int, label string) {
		t.Helper()
		if maxParallel == 0 {
			return
		}
		running := countStatus(jobs, actions_model.StatusRunning)
		waiting := countStatus(jobs, actions_model.StatusWaiting)
		success := countStatus(jobs, actions_model.StatusSuccess)
		remaining := len(jobs) - success
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
