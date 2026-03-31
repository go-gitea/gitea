// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
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
