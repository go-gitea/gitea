// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	runnerv1 "gitea.dev/actions-proto-go/runner/v1"

	"github.com/stretchr/testify/assert"
)

func TestStatusAsResult(t *testing.T) {
	cases := []struct {
		status Status
		want   runnerv1.Result
	}{
		{StatusUnknown, runnerv1.Result_RESULT_UNSPECIFIED},
		{StatusWaiting, runnerv1.Result_RESULT_UNSPECIFIED},
		{StatusRunning, runnerv1.Result_RESULT_UNSPECIFIED},
		{StatusBlocked, runnerv1.Result_RESULT_UNSPECIFIED},
		{StatusSuccess, runnerv1.Result_RESULT_SUCCESS},
		{StatusFailure, runnerv1.Result_RESULT_FAILURE},
		{StatusCancelled, runnerv1.Result_RESULT_CANCELLED},
		{StatusCancelling, runnerv1.Result_RESULT_CANCELLED},
		{StatusSkipped, runnerv1.Result_RESULT_SKIPPED},
	}

	for _, tt := range cases {
		assert.Equal(t, tt.want, tt.status.AsResult(), "status=%s", tt.status)
	}
}

func TestStatusFromResult(t *testing.T) {
	cases := []struct {
		result runnerv1.Result
		want   Status
	}{
		{runnerv1.Result_RESULT_UNSPECIFIED, StatusUnknown},
		{runnerv1.Result_RESULT_SUCCESS, StatusSuccess},
		{runnerv1.Result_RESULT_FAILURE, StatusFailure},
		{runnerv1.Result_RESULT_CANCELLED, StatusCancelled},
		{runnerv1.Result_RESULT_SKIPPED, StatusSkipped},
	}

	for _, tt := range cases {
		assert.Equal(t, tt.want, StatusFromResult(tt.result), "result=%s", tt.result)
	}
}

func newJob(status Status, continueOnError bool) *ActionRunJob {
	return &ActionRunJob{Status: status, ContinueOnError: continueOnError}
}

func TestAggregateJobStatusContinueOnError(t *testing.T) {
	cases := []struct {
		name string
		jobs []*ActionRunJob
		want Status
	}{
		{
			name: "all success",
			jobs: []*ActionRunJob{newJob(StatusSuccess, false), newJob(StatusSuccess, false)},
			want: StatusSuccess,
		},
		{
			name: "one failure without continue-on-error",
			jobs: []*ActionRunJob{newJob(StatusSuccess, false), newJob(StatusFailure, false)},
			want: StatusFailure,
		},
		{
			name: "one failure with continue-on-error",
			jobs: []*ActionRunJob{newJob(StatusSuccess, false), newJob(StatusFailure, true)},
			want: StatusSuccess,
		},
		{
			name: "only continued-failure",
			jobs: []*ActionRunJob{newJob(StatusFailure, true)},
			want: StatusSuccess,
		},
		{
			name: "continued-failure plus real failure",
			jobs: []*ActionRunJob{newJob(StatusFailure, true), newJob(StatusFailure, false)},
			want: StatusFailure,
		},
		{
			name: "all skipped",
			jobs: []*ActionRunJob{newJob(StatusSkipped, false), newJob(StatusSkipped, false)},
			want: StatusSkipped,
		},
		{
			name: "continued-failure plus skipped counts as success",
			jobs: []*ActionRunJob{newJob(StatusFailure, true), newJob(StatusSkipped, false)},
			want: StatusSuccess,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, AggregateJobStatus(tt.jobs))
		})
	}
}
