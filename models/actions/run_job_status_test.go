// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAggregateJobStatus(t *testing.T) {
	testStatuses := func(expected Status, statuses []Status) {
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
		// success with other status
		{[]Status{StatusSuccess}, StatusSuccess},
		{[]Status{StatusSuccess, StatusSkipped}, StatusSuccess}, // skipped doesn't affect success
		{[]Status{StatusSuccess, StatusFailure}, StatusFailure},
		{[]Status{StatusSuccess, StatusCancelled}, StatusCancelled},
		{[]Status{StatusSuccess, StatusWaiting}, StatusWaiting},
		{[]Status{StatusSuccess, StatusRunning}, StatusRunning},
		{[]Status{StatusSuccess, StatusBlocked}, StatusBlocked},

		// failure with other status, fail fast
		// Should "running" win? Maybe no: old code does make "running" win, but GitHub does fail fast.
		{[]Status{StatusFailure}, StatusFailure},
		{[]Status{StatusFailure, StatusSuccess}, StatusFailure},
		{[]Status{StatusFailure, StatusSkipped}, StatusFailure},
		{[]Status{StatusFailure, StatusCancelled}, StatusFailure},
		{[]Status{StatusFailure, StatusWaiting}, StatusFailure},
		{[]Status{StatusFailure, StatusRunning}, StatusFailure},
		{[]Status{StatusFailure, StatusBlocked}, StatusFailure},

		// skipped with other status
		{[]Status{StatusSkipped}, StatusSuccess},
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
