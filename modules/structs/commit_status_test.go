// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNoBetterThan(t *testing.T) {
	tests := []struct {
		s1, s2 CommitStatusState
		higher bool
	}{
		{CommitStatusError, CommitStatusFailure, true},
		{CommitStatusFailure, CommitStatusWarning, true},
		{CommitStatusWarning, CommitStatusPending, true},
		{CommitStatusPending, CommitStatusSuccess, true},
		{CommitStatusSuccess, CommitStatusSkipped, true},

		{CommitStatusError, "unknown-xxx", false},
		{"unknown-xxx", CommitStatusFailure, true},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.higher, tt.s1.HasHigherPriorityThan(tt.s2), "s1=%s, s2=%s, expected=%v", tt.s1, tt.s2, tt.higher)
	}
	assert.False(t, CommitStatusError.HasHigherPriorityThan(CommitStatusError))
}
