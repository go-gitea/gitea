// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jobparser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePriority(t *testing.T) {
	for value, want := range map[string]int{"high": PriorityHigh, "normal": PriorityNormal, "": PriorityNormal, "low": PriorityLow} {
		got, err := ParsePriority(value)
		require.NoError(t, err, value)
		assert.Equal(t, want, got, value)
	}
	for _, value := range []string{"High", "urgent", "1", "${{ vars.PRIORITY }}"} {
		got, err := ParsePriority(value)
		require.ErrorContains(t, err, "is not high, normal or low", value)
		assert.Equal(t, PriorityNormal, got, value)
	}
}

func TestValidateWorkflowStaticPriority(t *testing.T) {
	const jobs = "on: push\njobs:\n  a:\n    runs-on: x\n    steps:\n      - run: echo\n"
	for header, want := range map[string]string{
		"":                       "",
		"priority: high\n":       "",
		"priority: normal\n":     "",
		"priority: low\n":        "",
		"priority: urgent\n":     `priority: "urgent" is not high, normal or low`,
		"priority: 1\n":          `priority: "1" is not high, normal or low`,
		"priority: [high]\n":     "priority: expected one of high, normal or low",
		"priority: {level: 1}\n": "priority: expected one of high, normal or low",
	} {
		_, err := ValidateWorkflowStatic([]byte(header + jobs))
		if want == "" {
			assert.NoError(t, err, header)
		} else {
			assert.ErrorContains(t, err, want, header)
		}
	}
}

func TestParseKeepsPriorityInEveryJobPayload(t *testing.T) {
	content := []byte("priority: low\non: push\njobs:\n  a:\n    runs-on: x\n    steps:\n      - run: echo\n  b:\n    runs-on: x\n    strategy:\n      matrix:\n        n: [1, 2]\n    steps:\n      - run: echo\n")
	workflows, err := Parse(content)
	require.NoError(t, err)
	require.Len(t, workflows, 3)
	for _, swf := range workflows {
		assert.Equal(t, "low", swf.Priority)

		payload, err := swf.Marshal()
		require.NoError(t, err)
		stored, _, err := ParseRawSingleWorkflow(payload)
		require.NoError(t, err)
		assert.Equal(t, "low", stored.Priority, "a rerun or matrix expansion reads the stored payload")
	}
}
