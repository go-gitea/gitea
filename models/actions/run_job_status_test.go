// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAggregateJobStatus(t *testing.T) {
	testStatuses := func(expected Status, statuses []Status) {
		t.Helper()
		var jobs []*ActionRunJob
		for _, v := range statuses {
			jobs = append(jobs, &ActionRunJob{Status: v})
		}
		actual := AggregateJobStatus(jobs)
		if !assert.Equal(t, expected, actual) {
			var statusStrings []string
			for _, s := range statuses {
				statusStrings = append(statusStrings, s.String())
			}
			t.Errorf("AggregateJobStatus(%v) = %v, want %v", statusStrings, statusNames[actual], statusNames[expected])
		}
	}

	cases := []struct {
		statuses []Status
		expected Status
	}{
		// unknown cases, maybe it shouldn't happen in real world
		{[]Status{}, StatusUnknown},
		{[]Status{StatusUnknown, StatusSuccess}, StatusUnknown},
		{[]Status{StatusUnknown, StatusSkipped}, StatusUnknown},
		{[]Status{StatusUnknown, StatusFailure}, StatusFailure},
		{[]Status{StatusUnknown, StatusCancelled}, StatusCancelled},
		{[]Status{StatusUnknown, StatusWaiting}, StatusWaiting},
		{[]Status{StatusUnknown, StatusRunning}, StatusRunning},
		{[]Status{StatusUnknown, StatusBlocked}, StatusBlocked},

		// success with other status
		{[]Status{StatusSuccess}, StatusSuccess},
		{[]Status{StatusSuccess, StatusSkipped}, StatusSuccess}, // skipped doesn't affect success
		{[]Status{StatusSuccess, StatusFailure}, StatusFailure},
		{[]Status{StatusSuccess, StatusCancelled}, StatusCancelled},
		{[]Status{StatusSuccess, StatusWaiting}, StatusWaiting},
		{[]Status{StatusSuccess, StatusRunning}, StatusRunning},
		{[]Status{StatusSuccess, StatusBlocked}, StatusBlocked},

		// any cancelled, then cancelled
		{[]Status{StatusCancelled}, StatusCancelled},
		{[]Status{StatusCancelled, StatusSuccess}, StatusCancelled},
		{[]Status{StatusCancelled, StatusSkipped}, StatusCancelled},
		{[]Status{StatusCancelled, StatusFailure}, StatusCancelled},
		{[]Status{StatusCancelled, StatusWaiting}, StatusCancelled},
		{[]Status{StatusCancelled, StatusRunning}, StatusCancelled},
		{[]Status{StatusCancelled, StatusBlocked}, StatusCancelled},

		// failure with other status, fail fast
		// Should "running" win? Maybe no: old code does make "running" win, but GitHub does fail fast.
		{[]Status{StatusFailure}, StatusFailure},
		{[]Status{StatusFailure, StatusSuccess}, StatusFailure},
		{[]Status{StatusFailure, StatusSkipped}, StatusFailure},
		{[]Status{StatusFailure, StatusCancelled}, StatusCancelled},
		{[]Status{StatusFailure, StatusWaiting}, StatusFailure},
		{[]Status{StatusFailure, StatusRunning}, StatusFailure},
		{[]Status{StatusFailure, StatusBlocked}, StatusFailure},

		// skipped with other status
		// "all skipped" is also considered as "mergeable" by "services/actions.toCommitStatus", the same as GitHub
		{[]Status{StatusSkipped}, StatusSkipped},
		{[]Status{StatusSkipped, StatusSuccess}, StatusSuccess},
		{[]Status{StatusSkipped, StatusFailure}, StatusFailure},
		{[]Status{StatusSkipped, StatusCancelled}, StatusCancelled},
		{[]Status{StatusSkipped, StatusWaiting}, StatusWaiting},
		{[]Status{StatusSkipped, StatusRunning}, StatusRunning},
		{[]Status{StatusSkipped, StatusBlocked}, StatusBlocked},
	}

	for _, c := range cases {
		testStatuses(c.expected, c.statuses)
	}
}
