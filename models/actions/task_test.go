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
			name: "explicit name",
			jobStep: &jobparser.Step{
				Name: "Test Step",
			},
			expected: "Test Step",
		},
		{
			name: "uses step",
			jobStep: &jobparser.Step{
				Uses: "actions/checkout@v4",
			},
			expected: "actions/checkout@v4",
		},
		{
			name: "single-line run",
			jobStep: &jobparser.Step{
				Run: "echo hello",
			},
			expected: "Run echo hello",
		},
		{
			name: "multi-line run",
			jobStep: &jobparser.Step{
				Run: "echo hello\necho world",
			},
			expected: "Run echo hello",
		},
		{
			name: "multi-line run block scalar", // run: |\n  echo hello\n  echo world\n
			jobStep: &jobparser.Step{
				Run: "echo hello\necho world\n",
			},
			expected: "Run echo hello",
		},
		{
			name: "multi-line run with leading newline",
			jobStep: &jobparser.Step{
				Run: "\n  echo hello\n  echo world",
			},
			expected: "Run echo hello",
		},
		{
			name: "fallback to id",
			jobStep: &jobparser.Step{
				ID: "step-id",
			},
			expected: "step-id",
		},
		{
			name: "very long name",
			jobStep: &jobparser.Step{
				Name: "abcdeabcdeabcdeabcdeabcdeabcde",
			},
			expected: "abcdeabcdeabcdeab…",
		},
		{
			name: "very long run",
			jobStep: &jobparser.Step{
				Run: "abcdeabcdeabcdeabcdeabcdeabcde",
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
