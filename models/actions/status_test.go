// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
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
