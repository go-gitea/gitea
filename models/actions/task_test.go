// Copyright 2025 The Gitea Authors. All rights reserved.
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
		step     *jobparser.Step
		expected string
	}{
		{name: "uses without name", step: &jobparser.Step{Uses: "actions/checkout@v4"}, expected: "Run actions/checkout@v4"},
		{name: "run without name", step: &jobparser.Step{Run: "make build"}, expected: "Run make build"},
		{name: "run with explicit name", step: &jobparser.Step{Name: "Run tests", Run: "make test"}, expected: "Run tests"},
		{name: "uses with explicit name", step: &jobparser.Step{Name: "Checkout", Uses: "actions/checkout@v4"}, expected: "Checkout"},
		{name: "multi-command run without name", step: &jobparser.Step{Run: "echo hello && echo world"}, expected: "Run echo hello && echo world"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, makeTaskStepDisplayName(tt.step, 255))
		})
	}
}
