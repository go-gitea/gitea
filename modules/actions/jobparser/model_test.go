package jobparser

import (
	"fmt"
	"strings"
	"testing"

	"github.com/nektos/act/pkg/model"

	"github.com/stretchr/testify/assert"
)

// func TestParseRawOn(t *testing.T) {
// 	kases := []struct {
// 		input  string
// 		result []*Event
// 	}{
// 		{
// 			input: "on: issue_comment",
// 			result: []*Event{
// 				{
// 					Name: "issue_comment",
// 				},
// 			},
// 		},
// 		{
// 			input: "on:\n  push",
// 			result: []*Event{
// 				{
// 					Name: "push",
// 				},
// 			},
// 		},

// 		{
// 			input: "on:\n  - push\n  - pull_request",
// 			result: []*Event{
// 				{
// 					Name: "push",
// 				},
// 				{
// 					Name: "pull_request",
// 				},
// 			},
// 		},
// 		{
// 			input: "on:\n  push:\n    branches:\n      - master",
// 			result: []*Event{
// 				{
// 					Name: "push",
// 					acts: map[string][]string{
// 						"branches": {
// 							"master",
// 						},
// 					},
// 				},
// 			},
// 		},
// 		{
// 			input: "on:\n  branch_protection_rule:\n    types: [created, deleted]",
// 			result: []*Event{
// 				{
// 					Name: "branch_protection_rule",
// 					acts: map[string][]string{
// 						"types": {
// 							"created",
// 							"deleted",
// 						},
// 					},
// 				},
// 			},
// 		},
// 		{
// 			input: "on:\n  project:\n    types: [created, deleted]\n  milestone:\n    types: [opened, deleted]",
// 			result: []*Event{
// 				{
// 					Name: "project",
// 					acts: map[string][]string{
// 						"types": {
// 							"created",
// 							"deleted",
// 						},
// 					},
// 				},
// 				{
// 					Name: "milestone",
// 					acts: map[string][]string{
// 						"types": {
// 							"opened",
// 							"deleted",
// 						},
// 					},
// 				},
// 			},
// 		},
// 		{
// 			input: "on:\n  pull_request:\n    types:\n      - opened\n    branches:\n      - 'releases/**'",
// 			result: []*Event{
// 				{
// 					Name: "pull_request",
// 					acts: map[string][]string{
// 						"types": {
// 							"opened",
// 						},
// 						"branches": {
// 							"releases/**",
// 						},
// 					},
// 				},
// 			},
// 		},
// 		{
// 			input: "on:\n  push:\n    branches:\n      - main\n  pull_request:\n    types:\n      - opened\n    branches:\n      - '**'",
// 			result: []*Event{
// 				{
// 					Name: "push",
// 					acts: map[string][]string{
// 						"branches": {
// 							"main",
// 						},
// 					},
// 				},
// 				{
// 					Name: "pull_request",
// 					acts: map[string][]string{
// 						"types": {
// 							"opened",
// 						},
// 						"branches": {
// 							"**",
// 						},
// 					},
// 				},
// 			},
// 		},
// 		{
// 			input: "on:\n  push:\n    branches:\n      - 'main'\n      - 'releases/**'",
// 			result: []*Event{
// 				{
// 					Name: "push",
// 					acts: map[string][]string{
// 						"branches": {
// 							"main",
// 							"releases/**",
// 						},
// 					},
// 				},
// 			},
// 		},
// 		{
// 			input: "on:\n  push:\n    tags:\n      - v1.**",
// 			result: []*Event{
// 				{
// 					Name: "push",
// 					acts: map[string][]string{
// 						"tags": {
// 							"v1.**",
// 						},
// 					},
// 				},
// 			},
// 		},
// 		{
// 			input: "on: [pull_request, workflow_dispatch]",
// 			result: []*Event{
// 				{
// 					Name: "pull_request",
// 				},
// 				{
// 					Name: "workflow_dispatch",
// 				},
// 			},
// 		},
// 		{
// 			input: "on:\n  schedule:\n    - cron: '20 6 * * *'",
// 			result: []*Event{
// 				{
// 					Name: "schedule",
// 					schedules: []map[string]string{
// 						{
// 							"cron": "20 6 * * *",
// 						},
// 					},
// 				},
// 			},
// 		},
// 		{
// 			input: `on:
//   workflow_dispatch:
//     inputs:
//       logLevel:
//         description: 'Log level'
//         required: true
//         default: 'warning'
//         type: choice
//         options:
//         - info
//         - warning
//         - debug
//       tags:
//         description: 'Test scenario tags'
//         required: false
//         type: boolean
//       environment:
//         description: 'Environment to run tests against'
//         type: environment
//         required: true
//   push:
// `,
// 			result: []*Event{
// 				{
// 					Name: "workflow_dispatch",
// 					inputs: []WorkflowDispatchInput{
// 						{
// 							Name:        "logLevel",
// 							Description: "Log level",
// 							Required:    true,
// 							Default:     "warning",
// 							Type:        "choice",
// 							Options:     []string{"info", "warning", "debug"},
// 						},
// 						{
// 							Name:        "tags",
// 							Description: "Test scenario tags",
// 							Required:    false,
// 							Type:        "boolean",
// 						},
// 						{
// 							Name:        "environment",
// 							Description: "Environment to run tests against",
// 							Type:        "environment",
// 							Required:    true,
// 						},
// 					},
// 				},
// 				{
// 					Name: "push",
// 				},
// 			},
// 		},
// 	}
// 	for _, kase := range kases {
// 		t.Run(kase.input, func(t *testing.T) {
// 			origin, err := model.ReadWorkflow(strings.NewReader(kase.input))
// 			assert.NoError(t, err)

// 			events, err := ParseRawOn(&origin.RawOn)
// 			assert.NoError(t, err)
// 			assert.EqualValues(t, kase.result, events, fmt.Sprintf("%#v", events))
// 		})
// 	}
// }

// func TestSingleWorkflow_SetJob(t *testing.T) {
// 	t.Run("erase needs", func(t *testing.T) {
// 		content := ReadTestdata(t, "erase_needs.in.yaml")
// 		want := ReadTestdata(t, "erase_needs.out.yaml")
// 		swf, err := Parse(content)
// 		require.NoError(t, err)
// 		builder := &strings.Builder{}
// 		for _, v := range swf {
// 			id, job := v.Job()
// 			require.NoError(t, v.SetJob(id, job.EraseNeeds()))

// 			if builder.Len() > 0 {
// 				builder.WriteString("---\n")
// 			}
// 			encoder := yaml.NewEncoder(builder)
// 			encoder.SetIndent(2)
// 			require.NoError(t, encoder.Encode(v))
// 		}
// 		assert.Equal(t, string(want), builder.String())
// 	})
// }

func TestParseMappingNode(t *testing.T) {
	tests := []struct {
		input   string
		scalars []string
		datas   []interface{}
	}{
		{
			input:   "on:\n  push:\n    branches:\n      - master",
			scalars: []string{"push"},
			datas: []interface{}{
				map[string]interface{}{
					"branches": []interface{}{"master"},
				},
			},
		},
		{
			input:   "on:\n  branch_protection_rule:\n    types: [created, deleted]",
			scalars: []string{"branch_protection_rule"},
			datas: []interface{}{
				map[string]interface{}{
					"types": []interface{}{"created", "deleted"},
				},
			},
		},
		{
			input:   "on:\n  project:\n    types: [created, deleted]\n  milestone:\n    types: [opened, deleted]",
			scalars: []string{"project", "milestone"},
			datas: []interface{}{
				map[string]interface{}{
					"types": []interface{}{"created", "deleted"},
				},
				map[string]interface{}{
					"types": []interface{}{"opened", "deleted"},
				},
			},
		},
		{
			input:   "on:\n  pull_request:\n    types:\n      - opened\n    branches:\n      - 'releases/**'",
			scalars: []string{"pull_request"},
			datas: []interface{}{
				map[string]interface{}{
					"types":    []interface{}{"opened"},
					"branches": []interface{}{"releases/**"},
				},
			},
		},
		{
			input:   "on:\n  push:\n    branches:\n      - main\n  pull_request:\n    types:\n      - opened\n    branches:\n      - '**'",
			scalars: []string{"push", "pull_request"},
			datas: []interface{}{
				map[string]interface{}{
					"branches": []interface{}{"main"},
				},
				map[string]interface{}{
					"types":    []interface{}{"opened"},
					"branches": []interface{}{"**"},
				},
			},
		},
		{
			input:   "on:\n  schedule:\n    - cron: '20 6 * * *'",
			scalars: []string{"schedule"},
			datas: []interface{}{
				[]interface{}{map[string]interface{}{
					"cron": "20 6 * * *",
				}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			workflow, err := model.ReadWorkflow(strings.NewReader(test.input))
			assert.NoError(t, err)

			scalars, datas, err := parseMappingNode[interface{}](&workflow.RawOn)
			assert.NoError(t, err)
			assert.EqualValues(t, test.scalars, scalars, fmt.Sprintf("%#v", scalars))
			assert.EqualValues(t, test.datas, datas, fmt.Sprintf("%#v", datas))
		})
	}
}
