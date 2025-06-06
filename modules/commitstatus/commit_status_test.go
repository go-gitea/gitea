// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package commitstatus

import "testing"

func TestCombine(t *testing.T) {
	tests := []struct {
		name     string
		states   CommitStatusStates
		expected CommitStatusState
	}{
		// 0 states
		{
			name:     "empty",
			states:   CommitStatusStates{},
			expected: CommitStatusPending,
		},
		// 1 state
		{
			name:     "pending",
			states:   CommitStatusStates{CommitStatusPending},
			expected: CommitStatusPending,
		},
		{
			name:     "success",
			states:   CommitStatusStates{CommitStatusSuccess},
			expected: CommitStatusSuccess,
		},
		{
			name:     "error",
			states:   CommitStatusStates{CommitStatusError},
			expected: CommitStatusFailure,
		},
		{
			name:     "failure",
			states:   CommitStatusStates{CommitStatusFailure},
			expected: CommitStatusFailure,
		},
		{
			name:     "warning",
			states:   CommitStatusStates{CommitStatusWarning},
			expected: CommitStatusSuccess,
		},
		// 2 states
		{
			name:     "pending and success",
			states:   CommitStatusStates{CommitStatusPending, CommitStatusSuccess},
			expected: CommitStatusPending,
		},
		{
			name:     "pending and error",
			states:   CommitStatusStates{CommitStatusPending, CommitStatusError},
			expected: CommitStatusFailure,
		},
		{
			name:     "pending and failure",
			states:   CommitStatusStates{CommitStatusPending, CommitStatusFailure},
			expected: CommitStatusFailure,
		},
		{
			name:     "pending and warning",
			states:   CommitStatusStates{CommitStatusPending, CommitStatusWarning},
			expected: CommitStatusPending,
		},
		{
			name:     "success and error",
			states:   CommitStatusStates{CommitStatusSuccess, CommitStatusError},
			expected: CommitStatusFailure,
		},
		{
			name:     "success and failure",
			states:   CommitStatusStates{CommitStatusSuccess, CommitStatusFailure},
			expected: CommitStatusFailure,
		},
		{
			name:     "success and warning",
			states:   CommitStatusStates{CommitStatusSuccess, CommitStatusWarning},
			expected: CommitStatusSuccess,
		},
		{
			name:     "error and failure",
			states:   CommitStatusStates{CommitStatusError, CommitStatusFailure},
			expected: CommitStatusFailure,
		},
		{
			name:     "error and warning",
			states:   CommitStatusStates{CommitStatusError, CommitStatusWarning},
			expected: CommitStatusFailure,
		},
		{
			name:     "failure and warning",
			states:   CommitStatusStates{CommitStatusFailure, CommitStatusWarning},
			expected: CommitStatusFailure,
		},
		// 3 states
		{
			name:     "pending, success and warning",
			states:   CommitStatusStates{CommitStatusPending, CommitStatusSuccess, CommitStatusWarning},
			expected: CommitStatusPending,
		},
		{
			name:     "pending, success and error",
			states:   CommitStatusStates{CommitStatusPending, CommitStatusSuccess, CommitStatusError},
			expected: CommitStatusFailure,
		},
		{
			name:     "pending, success and failure",
			states:   CommitStatusStates{CommitStatusPending, CommitStatusSuccess, CommitStatusFailure},
			expected: CommitStatusFailure,
		},
		{
			name:     "pending, error and failure",
			states:   CommitStatusStates{CommitStatusPending, CommitStatusError, CommitStatusFailure},
			expected: CommitStatusFailure,
		},
		{
			name:     "success, error and warning",
			states:   CommitStatusStates{CommitStatusSuccess, CommitStatusError, CommitStatusWarning},
			expected: CommitStatusFailure,
		},
		{
			name:     "success, failure and warning",
			states:   CommitStatusStates{CommitStatusSuccess, CommitStatusFailure, CommitStatusWarning},
			expected: CommitStatusFailure,
		},
		{
			name:     "error, failure and warning",
			states:   CommitStatusStates{CommitStatusError, CommitStatusFailure, CommitStatusWarning},
			expected: CommitStatusFailure,
		},
		{
			name:     "success, warning and skipped",
			states:   CommitStatusStates{CommitStatusSuccess, CommitStatusWarning, CommitStatusSkipped},
			expected: CommitStatusSuccess,
		},
		// All success
		{
			name:     "all success",
			states:   CommitStatusStates{CommitStatusSuccess, CommitStatusSuccess, CommitStatusSuccess},
			expected: CommitStatusSuccess,
		},
		// All pending
		{
			name:     "all pending",
			states:   CommitStatusStates{CommitStatusPending, CommitStatusPending, CommitStatusPending},
			expected: CommitStatusPending,
		},
		{
			name:     "all skipped",
			states:   CommitStatusStates{CommitStatusSkipped, CommitStatusSkipped, CommitStatusSkipped},
			expected: CommitStatusSuccess,
		},
		// 4 states
		{
			name:     "pending, success, error and warning",
			states:   CommitStatusStates{CommitStatusPending, CommitStatusSuccess, CommitStatusError, CommitStatusWarning},
			expected: CommitStatusFailure,
		},
		{
			name:     "pending, success, failure and warning",
			states:   CommitStatusStates{CommitStatusPending, CommitStatusSuccess, CommitStatusFailure, CommitStatusWarning},
			expected: CommitStatusFailure,
		},
		{
			name:     "pending, error, failure and warning",
			states:   CommitStatusStates{CommitStatusPending, CommitStatusError, CommitStatusFailure, CommitStatusWarning},
			expected: CommitStatusFailure,
		},
		{
			name:     "success, error, failure and warning",
			states:   CommitStatusStates{CommitStatusSuccess, CommitStatusError, CommitStatusFailure, CommitStatusWarning},
			expected: CommitStatusFailure,
		},
		{
			name:     "mixed states",
			states:   CommitStatusStates{CommitStatusPending, CommitStatusSuccess, CommitStatusError, CommitStatusWarning},
			expected: CommitStatusFailure,
		},
		{
			name:     "mixed states with all success",
			states:   CommitStatusStates{CommitStatusSuccess, CommitStatusSuccess, CommitStatusPending, CommitStatusWarning},
			expected: CommitStatusPending,
		},
		{
			name:     "all success with warning",
			states:   CommitStatusStates{CommitStatusSuccess, CommitStatusSuccess, CommitStatusSuccess, CommitStatusWarning},
			expected: CommitStatusSuccess,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.states.Combine()
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
