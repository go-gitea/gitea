// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"github.com/nektos/act/pkg/jobparser"
	"github.com/stretchr/testify/assert"
)

func TestMakeTaskStepDisplayName(t *testing.T) {
	tests := []struct {
		name     string
		jobStep  *jobparser.Step
		expected string
	}{
		{
			name: "with name",
			jobStep: &jobparser.Step{
				Name: "Test Step",
			},
			expected: "Test Step",
		},
		{
			name: "without name",
			jobStep: &jobparser.Step{
				ID: "test-step-id",
			},
			expected: "Run test-step-id",
		},
		{
			name: "very long name",
			jobStep: &jobparser.Step{
				Name: "abcdeabcdeabcdeabcdeabcdeabcde",
			},
			expected: "abcdeabcdeabcdeab…",
		},
		{
			name: "very long id",
			jobStep: &jobparser.Step{
				ID: "abcdeabcdeabcdeabcdeabcdeabcde",
			},
			expected: "Run abcdeabcdeabc…",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := makeTaskStepDisplayName(tt.jobStep, 20)
			assert.Equal(t, tt.expected, result)
		})
	}
}
