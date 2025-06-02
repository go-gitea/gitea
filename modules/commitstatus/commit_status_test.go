// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package commitstatus

import "testing"

func TestCombine(t *testing.T) {
	tests := []struct {
		name     string
		states   CommitStatusStates
		expected CombinedStatusState
	}{
		// 0 states
		{
			name:     "empty",
			states:   CommitStatusStates{},
			expected: CombinedStatusPending,
		},
		// 1 state
		{
			name:     "pending",
			states:   CommitStatusStates{CommitStatusPending},
			expected: CombinedStatusPending,
		},
		{
			name:     "success",
			states:   CommitStatusStates{CommitStatusSuccess},
			expected: CombinedStatusSuccess,
		},
		{
			name:     "error",
			states:   CommitStatusStates{CommitStatusError},
			expected: CombinedStatusFailure,
		},
		{
			name:     "failure",
			states:   CommitStatusStates{CommitStatusFailure},
			expected: CombinedStatusFailure,
		},
		{
			name:     "warning",
			states:   CommitStatusStates{CommitStatusWarning},
			expected: CombinedStatusSuccess,
		},
		// 2 states
		{
			name:     "pending and success",
			states:   CommitStatusStates{CommitStatusPending, CommitStatusSuccess},
			expected: CombinedStatusPending,
		},
		{
			name:     "pending and error",
			states:   CommitStatusStates{CommitStatusPending, CommitStatusError},
			expected: CombinedStatusFailure,
		},
		{
			name:     "pending and failure",
			states:   CommitStatusStates{CommitStatusPending, CommitStatusFailure},
			expected: CombinedStatusFailure,
		},
		{
			name:     "pending and warning",
			states:   CommitStatusStates{CommitStatusPending, CommitStatusWarning},
			expected: CombinedStatusPending,
		},
		{
			name:     "success and error",
			states:   CommitStatusStates{CommitStatusSuccess, CommitStatusError},
			expected: CombinedStatusFailure,
		},
		{
			name:     "success and failure",
			states:   CommitStatusStates{CommitStatusSuccess, CommitStatusFailure},
			expected: CombinedStatusFailure,
		},
		{
			name:     "success and warning",
			states:   CommitStatusStates{CommitStatusSuccess, CommitStatusWarning},
			expected: CombinedStatusSuccess,
		},
		{
			name:     "error and failure",
			states:   CommitStatusStates{CommitStatusError, CommitStatusFailure},
			expected: CombinedStatusFailure,
		},
		{
			name:     "error and warning",
			states:   CommitStatusStates{CommitStatusError, CommitStatusWarning},
			expected: CombinedStatusFailure,
		},
		{
			name:     "failure and warning",
			states:   CommitStatusStates{CommitStatusFailure, CommitStatusWarning},
			expected: CombinedStatusFailure,
		},
		// 3 states
		{
			name:     "pending, success and warning",
			states:   CommitStatusStates{CommitStatusPending, CommitStatusSuccess, CommitStatusWarning},
			expected: CombinedStatusPending,
		},
		{
			name:     "pending, success and error",
			states:   CommitStatusStates{CommitStatusPending, CommitStatusSuccess, CommitStatusError},
			expected: CombinedStatusFailure,
		},
		{
			name:     "pending, success and failure",
			states:   CommitStatusStates{CommitStatusPending, CommitStatusSuccess, CommitStatusFailure},
			expected: CombinedStatusFailure,
		},
		{
			name:     "pending, error and failure",
			states:   CommitStatusStates{CommitStatusPending, CommitStatusError, CommitStatusFailure},
			expected: CombinedStatusFailure,
		},
		{
			name:     "success, error and warning",
			states:   CommitStatusStates{CommitStatusSuccess, CommitStatusError, CommitStatusWarning},
			expected: CombinedStatusFailure,
		},
		{
			name:     "success, failure and warning",
			states:   CommitStatusStates{CommitStatusSuccess, CommitStatusFailure, CommitStatusWarning},
			expected: CombinedStatusFailure,
		},
		{
			name:     "error, failure and warning",
			states:   CommitStatusStates{CommitStatusError, CommitStatusFailure, CommitStatusWarning},
			expected: CombinedStatusFailure,
		},
		// All success
		{
			name:     "all success",
			states:   CommitStatusStates{CommitStatusSuccess, CommitStatusSuccess, CommitStatusSuccess},
			expected: CombinedStatusSuccess,
		},
		// All pending
		{
			name:     "all pending",
			states:   CommitStatusStates{CommitStatusPending, CommitStatusPending, CommitStatusPending},
			expected: CombinedStatusPending,
		},
		// 4 states
		{
			name:     "pending, success, error and warning",
			states:   CommitStatusStates{CommitStatusPending, CommitStatusSuccess, CommitStatusError, CommitStatusWarning},
			expected: CombinedStatusFailure,
		},
		{
			name:     "pending, success, failure and warning",
			states:   CommitStatusStates{CommitStatusPending, CommitStatusSuccess, CommitStatusFailure, CommitStatusWarning},
			expected: CombinedStatusFailure,
		},
		{
			name:     "pending, error, failure and warning",
			states:   CommitStatusStates{CommitStatusPending, CommitStatusError, CommitStatusFailure, CommitStatusWarning},
			expected: CombinedStatusFailure,
		},
		{
			name:     "success, error, failure and warning",
			states:   CommitStatusStates{CommitStatusSuccess, CommitStatusError, CommitStatusFailure, CommitStatusWarning},
			expected: CombinedStatusFailure,
		},
		{
			name:     "mixed states",
			states:   CommitStatusStates{CommitStatusPending, CommitStatusSuccess, CommitStatusError, CommitStatusWarning},
			expected: CombinedStatusFailure,
		},
		{
			name:     "mixed states with all success",
			states:   CommitStatusStates{CommitStatusSuccess, CommitStatusSuccess, CommitStatusPending, CommitStatusWarning},
			expected: CombinedStatusPending,
		},
		{
			name:     "all success with warning",
			states:   CommitStatusStates{CommitStatusSuccess, CommitStatusSuccess, CommitStatusSuccess, CommitStatusWarning},
			expected: CombinedStatusSuccess,
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
