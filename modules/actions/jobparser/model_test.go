// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jobparser

import (
	"strings"
	"testing"

	"github.com/nektos/act/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestGetEvents(t *testing.T) {
	content := `
name: My Workflow

on:
  push:
    branches: [main]
  schedule:
    - cron: '0 0 * * *'
  workflow_dispatch:
    inputs:
      my_variable1:
        description: 'first variable'
        required: true
        type: string
      my_variable2:
        description: 'second variable'
        required: false
        type: number
        default: 4

jobs:
  example:
    runs-on: ubuntu-latest
    steps:
      - run: exit 0
`
	expected := make([]*Event, 3)
	expected[0] = &Event{acts: map[string][]string{"branches": {"main"}}, Name: "push"}
	expected[1] = &Event{Name: "schedule", schedules: []map[string]string{{"cron": "0 0 * * *"}}}
	expected[2] = &Event{
		Name: "workflow_dispatch",
		inputs: []WorkflowDispatchInput{
			{
				Name:        "my_variable1",
				Description: "first variable",
				Required:    true,
				Type:        "string",
			},
			{
				Name:        "my_variable2",
				Type:        "number",
				Description: "second variable",
				Default:     "4",
			},
		},
	}
	actual, err := GetEventsFromContent([]byte(content))
	require.NoError(t, err)
	assert.Len(t, actual, 3)
	assert.Equal(t, expected, actual)
	//Toggler
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
			assert.EqualValues(t, test.scalars, scalars, "%#v", scalars)
			assert.EqualValues(t, test.datas, datas, "%#v", datas)
		})
	}
}
