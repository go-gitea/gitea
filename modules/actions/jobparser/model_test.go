// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jobparser

import (
	"strings"
	"testing"

	"github.com/nektos/act/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"
)

func TestParseRawOn(t *testing.T) {
	kases := []struct {
		input  string
		result []*Event
	}{
		{
			input: "on: issue_comment",
			result: []*Event{
				{
					Name: "issue_comment",
				},
			},
		},
		{
			input: "on:\n  push",
			result: []*Event{
				{
					Name: "push",
				},
			},
		},

		{
			input: "on:\n  - push\n  - pull_request",
			result: []*Event{
				{
					Name: "push",
				},
				{
					Name: "pull_request",
				},
			},
		},
		{
			input: "on:\n  push:\n    branches:\n      - master",
			result: []*Event{
				{
					Name: "push",
					acts: map[string][]string{
						"branches": {
							"master",
						},
					},
				},
			},
		},
		{
			input: "on:\n  push:\n    branches: main",
			result: []*Event{
				{
					Name: "push",
					acts: map[string][]string{
						"branches": {
							"main",
						},
					},
				},
			},
		},
		{
			input: "on:\n  branch_protection_rule:\n    types: [created, deleted]",
			result: []*Event{
				{
					Name: "branch_protection_rule",
					acts: map[string][]string{
						"types": {
							"created",
							"deleted",
						},
					},
				},
			},
		},
		{
			input: "on:\n  project:\n    types: [created, deleted]\n  milestone:\n    types: [opened, deleted]",
			result: []*Event{
				{
					Name: "project",
					acts: map[string][]string{
						"types": {
							"created",
							"deleted",
						},
					},
				},
				{
					Name: "milestone",
					acts: map[string][]string{
						"types": {
							"opened",
							"deleted",
						},
					},
				},
			},
		},
		{
			input: "on:\n  pull_request:\n    types:\n      - opened\n    branches:\n      - 'releases/**'",
			result: []*Event{
				{
					Name: "pull_request",
					acts: map[string][]string{
						"types": {
							"opened",
						},
						"branches": {
							"releases/**",
						},
					},
				},
			},
		},
		{
			input: "on:\n  push:\n    branches:\n      - main\n  pull_request:\n    types:\n      - opened\n    branches:\n      - '**'",
			result: []*Event{
				{
					Name: "push",
					acts: map[string][]string{
						"branches": {
							"main",
						},
					},
				},
				{
					Name: "pull_request",
					acts: map[string][]string{
						"types": {
							"opened",
						},
						"branches": {
							"**",
						},
					},
				},
			},
		},
		{
			input: "on:\n  push:\n    branches:\n      - 'main'\n      - 'releases/**'",
			result: []*Event{
				{
					Name: "push",
					acts: map[string][]string{
						"branches": {
							"main",
							"releases/**",
						},
					},
				},
			},
		},
		{
			input: "on:\n  push:\n    tags:\n      - v1.**",
			result: []*Event{
				{
					Name: "push",
					acts: map[string][]string{
						"tags": {
							"v1.**",
						},
					},
				},
			},
		},
		{
			input: "on: [pull_request, workflow_dispatch]",
			result: []*Event{
				{
					Name: "pull_request",
				},
				{
					Name: "workflow_dispatch",
				},
			},
		},
		{
			input: "on:\n  schedule:\n    - cron: '20 6 * * *'",
			result: []*Event{
				{
					Name: "schedule",
					schedules: []map[string]string{
						{
							"cron": "20 6 * * *",
						},
					},
				},
			},
		},
		{
			input: `on:
  workflow_dispatch:
    inputs:
      logLevel:
        description: 'Log level'
        required: true
        default: 'warning'
        type: choice
        options:
        - info
        - warning
        - debug
      tags:
        description: 'Test scenario tags'
        required: false
        type: boolean
      environment:
        description: 'Environment to run tests against'
        type: environment
        required: true
  push:
`,
			result: []*Event{
				{
					Name: "workflow_dispatch",
					inputs: []WorkflowDispatchInput{
						{
							Name:        "logLevel",
							Description: "Log level",
							Required:    true,
							Default:     "warning",
							Type:        "choice",
							Options:     []string{"info", "warning", "debug"},
						},
						{
							Name:        "tags",
							Description: "Test scenario tags",
							Required:    false,
							Type:        "boolean",
						},
						{
							Name:        "environment",
							Description: "Environment to run tests against",
							Type:        "environment",
							Required:    true,
						},
					},
				},
				{
					Name: "push",
				},
			},
		},
	}
	for _, kase := range kases {
		t.Run(kase.input, func(t *testing.T) {
			origin, err := model.ReadWorkflow(strings.NewReader(kase.input))
			assert.NoError(t, err)

			events, err := ParseRawOn(&origin.RawOn)
			assert.NoError(t, err)
			assert.Equal(t, kase.result, events, events)
		})
	}
}

func TestSingleWorkflow_SetJob(t *testing.T) {
	t.Run("erase needs", func(t *testing.T) {
		content := ReadTestdata(t, "erase_needs.in.yaml")
		want := ReadTestdata(t, "erase_needs.out.yaml")
		swf, err := Parse(content)
		require.NoError(t, err)
		builder := &strings.Builder{}
		for _, v := range swf {
			id, job := v.Job()
			require.NoError(t, v.SetJob(id, job.EraseNeeds()))

			if builder.Len() > 0 {
				builder.WriteString("---\n")
			}
			encoder := yaml.NewEncoder(builder)
			encoder.SetIndent(2)
			require.NoError(t, encoder.Encode(v))
		}
		assert.Equal(t, string(want), builder.String())
	})
}

func TestParseMappingNode(t *testing.T) {
	tests := []struct {
		input   string
		scalars []string
		datas   []any
	}{
		{
			input:   "on:\n  push:\n    branches:\n      - master",
			scalars: []string{"push"},
			datas: []any{
				map[string]any{
					"branches": []any{"master"},
				},
			},
		},
		{
			input:   "on:\n  branch_protection_rule:\n    types: [created, deleted]",
			scalars: []string{"branch_protection_rule"},
			datas: []any{
				map[string]any{
					"types": []any{"created", "deleted"},
				},
			},
		},
		{
			input:   "on:\n  project:\n    types: [created, deleted]\n  milestone:\n    types: [opened, deleted]",
			scalars: []string{"project", "milestone"},
			datas: []any{
				map[string]any{
					"types": []any{"created", "deleted"},
				},
				map[string]any{
					"types": []any{"opened", "deleted"},
				},
			},
		},
		{
			input:   "on:\n  pull_request:\n    types:\n      - opened\n    branches:\n      - 'releases/**'",
			scalars: []string{"pull_request"},
			datas: []any{
				map[string]any{
					"types":    []any{"opened"},
					"branches": []any{"releases/**"},
				},
			},
		},
		{
			input:   "on:\n  push:\n    branches:\n      - main\n  pull_request:\n    types:\n      - opened\n    branches:\n      - '**'",
			scalars: []string{"push", "pull_request"},
			datas: []any{
				map[string]any{
					"branches": []any{"main"},
				},
				map[string]any{
					"types":    []any{"opened"},
					"branches": []any{"**"},
				},
			},
		},
		{
			input:   "on:\n  schedule:\n    - cron: '20 6 * * *'",
			scalars: []string{"schedule"},
			datas: []any{
				[]any{map[string]any{
					"cron": "20 6 * * *",
				}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			workflow, err := model.ReadWorkflow(strings.NewReader(test.input))
			assert.NoError(t, err)

			scalars, datas, err := parseMappingNode[any](&workflow.RawOn)
			assert.NoError(t, err)
			assert.Equal(t, test.scalars, scalars, scalars)
			assert.Equal(t, test.datas, datas, datas)
		})
	}
}
