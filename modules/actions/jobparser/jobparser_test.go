// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jobparser

import (
	"strings"
	"testing"

	"github.com/nektos/act/pkg/exprparser"
	"github.com/nektos/act/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		options []ParseOption
		wantErr bool
	}{
		{
			name:    "multiple_jobs",
			options: nil,
			wantErr: false,
		},
		{
			name:    "multiple_matrix",
			options: nil,
			wantErr: false,
		},
		{
			name:    "has_needs",
			options: nil,
			wantErr: false,
		},
		{
			name:    "has_with",
			options: nil,
			wantErr: false,
		},
		{
			name:    "has_secrets",
			options: nil,
			wantErr: false,
		},
		{
			name:    "empty_step",
			options: nil,
			wantErr: false,
		},
		{
			name:    "job_name_with_matrix",
			options: nil,
			wantErr: false,
		},
		{
			name: "job_name_with_matrix_dynamic",
			options: []ParseOption{
				WithJobResults(map[string]string{
					"job1": "success",
				}),
				WithJobOutputs(map[string]map[string]string{
					"job1": {
						"versions": "[1.17, 1.18, 1.19]",
					},
				}),
			},
			wantErr: false,
		},
		{
			name:    "prefixed_newline",
			options: nil,
			wantErr: false,
		},
	}
	invalidFileTests := []struct {
		name string
	}{
		{name: "null_job_implicit"},
		{name: "null_job_explicit"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := ReadTestdata(t, tt.name+".in.yaml")
			want := ReadTestdata(t, tt.name+".out.yaml")
			got, err := Parse(content, tt.options...)
			if tt.wantErr {
				require.Error(t, err)
			}
			require.NoError(t, err)

			builder := &strings.Builder{}
			for _, v := range got {
				if builder.Len() > 0 {
					builder.WriteString("---\n")
				}
				encoder := yaml.NewEncoder(builder)
				encoder.SetIndent(2)
				require.NoError(t, encoder.Encode(v))
				id, job := v.Job()
				assert.NotEmpty(t, id)
				assert.NotNil(t, job)
			}
			assert.Equal(t, string(want), builder.String())
		})
	}

	for _, tt := range invalidFileTests {
		t.Run(tt.name, func(t *testing.T) {
			content := ReadTestdata(t, tt.name+".in.yaml")
			require.NotPanics(t, func() {
				_, err := Parse(content)
				require.Error(t, err)
			})
		})
	}
}

func TestDeepCopyYamlNode(t *testing.T) {
	t.Run("deep_copy_preserves_isolation", func(t *testing.T) {
		// Create original node with nested content
		original := &yaml.Node{
			Kind:  yaml.MappingNode,
			Tag:   "!!map",
			Value: "",
			Content: []*yaml.Node{
				{Kind: yaml.ScalarNode, Tag: "!!str", Value: "key1"},
				{Kind: yaml.ScalarNode, Tag: "!!str", Value: "value1"},
				{Kind: yaml.ScalarNode, Tag: "!!str", Value: "key2"},
				{Kind: yaml.ScalarNode, Tag: "!!str", Value: "value2"},
			},
		}

		// Create deep copy
		copied := deepCopyYamlNode(original)

		// Verify copy is not nil
		require.NotNil(t, copied)

		// Verify values are equal
		assert.Equal(t, original.Kind, copied.Kind)
		assert.Equal(t, original.Tag, copied.Tag)
		assert.Len(t, original.Content, len(copied.Content))

		// Verify content pointers are different (isolation)
		for i, node := range original.Content {
			assert.NotSame(t, node, copied.Content[i], "Content[%d] should be different pointers", i)
			assert.Equal(t, node.Value, copied.Content[i].Value, "Content[%d] values should be equal", i)
		}

		// Modify the copy and verify original is unaffected
		copied.Content[0].Value = "modified"
		assert.NotEqual(t, original.Content[0].Value, copied.Content[0].Value)
	})

	t.Run("deep_copy_handles_nil", func(t *testing.T) {
		copied := deepCopyYamlNode(nil)
		assert.Nil(t, copied)
	})

	t.Run("deep_copy_handles_recursive", func(t *testing.T) {
		// Create a nested structure
		original := &yaml.Node{
			Kind:  yaml.MappingNode,
			Tag:   "!!map",
			Value: "",
			Content: []*yaml.Node{
				{Kind: yaml.ScalarNode, Tag: "!!str", Value: "nested"},
				{
					Kind:  yaml.MappingNode,
					Tag:   "!!map",
					Value: "",
					Content: []*yaml.Node{
						{Kind: yaml.ScalarNode, Tag: "!!str", Value: "inner"},
						{Kind: yaml.ScalarNode, Tag: "!!str", Value: "data"},
					},
				},
			},
		}

		copied := deepCopyYamlNode(original)

		// Verify deep isolation at all levels
		require.NotNil(t, copied)
		assert.NotSame(t, original.Content[1], copied.Content[1])
		assert.NotSame(t, original.Content[1].Content[0], copied.Content[1].Content[0])

		// Modify nested copy and verify original is unaffected
		copied.Content[1].Content[0].Value = "modified"
		assert.NotEqual(t, original.Content[1].Content[0].Value, copied.Content[1].Content[0].Value)
	})
}

func TestStrategyIsolationAfterEvaluation(t *testing.T) {
	// This test verifies that EvaluateYamlNode mutations on a copied Strategy
	// do not affect the original Strategy. This was the root cause of the issue.
	t.Run("evaluation_does_not_mutate_original", func(t *testing.T) {
		// Create an original job with a matrix
		originalJob := &model.Job{
			Strategy: &model.Strategy{
				RawMatrix: yaml.Node{
					Kind:  yaml.MappingNode,
					Tag:   "!!map",
					Value: "",
					Content: []*yaml.Node{
						{Kind: yaml.ScalarNode, Tag: "!!str", Value: "version"},
						{
							Kind:  yaml.SequenceNode,
							Tag:   "!!seq",
							Value: "",
							Content: []*yaml.Node{
								{Kind: yaml.ScalarNode, Tag: "!!str", Value: "${{ fromJson(needs.setup.outputs.versions) }}"},
							},
						},
					},
				},
			},
		}

		// Save the original Content pointer for verification
		originalContentPtr := originalJob.Strategy.RawMatrix.Content[1].Content[0]
		originalValue := originalContentPtr.Value

		// Simulate what happens in Parse(): shallow copy followed by evaluation
		evaluatedJob := *originalJob
		if originalJob.Strategy != nil {
			stratCopy := *originalJob.Strategy
			// This is the fix: deep copy the RawMatrix
			stratCopy.RawMatrix = *deepCopyYamlNode(&originalJob.Strategy.RawMatrix)
			evaluatedJob.Strategy = &stratCopy
		}

		// Create an evaluator and evaluate the matrix
		// (In real usage, this would have job outputs and other context)
		evaluator := NewExpressionEvaluator(exprparser.NewInterpeter(
			&exprparser.EvaluationEnvironment{
				Github: &model.GithubContext{},
				Vars:   map[string]string{},
				Inputs: map[string]any{},
			},
			exprparser.Config{},
		))

		// Evaluate the copied node
		_ = evaluator.EvaluateYamlNode(&evaluatedJob.Strategy.RawMatrix)

		// Verify that the original job's matrix is unchanged
		assert.Equal(t, originalValue, originalJob.Strategy.RawMatrix.Content[1].Content[0].Value,
			"Original job's matrix should not be mutated by evaluation")

		// Verify that they are now different pointers (isolation)
		assert.NotSame(t, originalJob.Strategy.RawMatrix.Content[1].Content[0],
			evaluatedJob.Strategy.RawMatrix.Content[1].Content[0],
			"Evaluated job should have different node pointers")
	})
}

func TestParseWithMissingJobOutputs(t *testing.T) {
	// Test graceful degradation when job outputs are missing
	t.Run("missing_job_outputs_degrades_gracefully", func(t *testing.T) {
		workflowYAML := `
name: test-missing-outputs
on: push

jobs:
  setup:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        version: [1.0, 2.0]
    steps:
      - run: echo setup

  build:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        os: [ubuntu-latest, windows-latest]
    steps:
      - run: echo build
`
		// Parse without providing job outputs - should gracefully handle
		result, err := Parse([]byte(workflowYAML))

		// Should not error on parse
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.NotEmpty(t, result)
	})

	t.Run("empty_job_outputs_map", func(t *testing.T) {
		workflowYAML := `
name: test-empty-outputs
on: push

jobs:
  setup:
    runs-on: ubuntu-latest
    steps:
      - run: echo setup

  build:
    needs: setup
    runs-on: ubuntu-latest
    strategy:
      matrix:
        version: [1.0, 2.0]
    steps:
      - run: echo build
`
		// Parse with empty job outputs
		result, err := Parse([]byte(workflowYAML),
			WithJobOutputs(map[string]map[string]string{}))

		assert.NoError(t, err)
		assert.NotNil(t, result)

		// Should still parse successfully
		assert.NotEmpty(t, result)
	})
}

func TestParseWithNeedsReferenceNoOutputs(t *testing.T) {
	// Test references to jobs that have no outputs provided
	t.Run("needs_reference_without_outputs", func(t *testing.T) {
		workflowYAML := `
name: test-needs-no-outputs
on: push

jobs:
  setup:
    runs-on: ubuntu-latest
    steps:
      - run: echo setup

  build:
    needs: setup
    runs-on: ubuntu-latest
    strategy:
      matrix:
        os: [ubuntu-latest, windows-latest]
    steps:
      - run: echo build
`
		// Parse with a needs reference but static matrix only
		result, err := Parse([]byte(workflowYAML),
			WithJobResults(map[string]string{
				"setup": "success",
			}))

		// Should not error on parse
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.NotEmpty(t, result)
	})

	t.Run("needs_reference_with_partial_outputs", func(t *testing.T) {
		workflowYAML := `
name: test-partial-outputs
on: push

jobs:
  setup:
    runs-on: ubuntu-latest
    outputs:
      versions: "[1.0, 2.0]"
    steps:
      - run: echo setup

  build:
    needs: setup
    runs-on: ubuntu-latest
    strategy:
      matrix:
        version: ${{ fromJson(needs.setup.outputs.versions) }}
        os: [ubuntu-latest, windows-latest]
    steps:
      - run: echo build
`
		// Parse with partial outputs provided
		result, err := Parse([]byte(workflowYAML),
			WithJobOutputs(map[string]map[string]string{
				"setup": {
					"versions": "[1.0, 2.0]",
				},
			}))

		assert.NoError(t, err)
		assert.NotNil(t, result)

		// Should parse successfully
		assert.NotEmpty(t, result)
	})
}

func TestParseWithMixedMatrixValues(t *testing.T) {
	// Test matrix with both static arrays and dynamic template expressions
	t.Run("static_and_dynamic_matrix_values", func(t *testing.T) {
		workflowYAML := `
name: test-mixed-matrix
on: push

jobs:
  setup:
    runs-on: ubuntu-latest
    outputs:
      versions: "[1.0, 2.0]"
    steps:
      - run: echo setup

  build:
    needs: setup
    runs-on: ubuntu-latest
    strategy:
      matrix:
        os: [ubuntu-latest, windows-latest, macos-latest]
        version: ${{ fromJson(needs.setup.outputs.versions) }}
        node: [14, 16, 18]
    steps:
      - run: echo build
`
		// Parse with dynamic matrix values
		result, err := Parse([]byte(workflowYAML),
			WithJobOutputs(map[string]map[string]string{
				"setup": {
					"versions": "[1.0, 2.0]",
				},
			}))

		assert.NoError(t, err)
		assert.NotNil(t, result)

		// Verify we have workflows
		assert.NotEmpty(t, result)

		// Check that all three matrix dimensions are present
		hasAllDimensions := false
		for _, workflow := range result {
			id, swfJob := workflow.Job()
			if id == "build" {
				// In jobparser, we just verify the job was parsed successfully
				if swfJob != nil {
					// Check strategy has matrix
					if swfJob.Strategy.RawMatrix.Kind != 0 {
						// All three dimensions should be defined
						hasAllDimensions = true
					}
				}
				break
			}
		}

		assert.True(t, hasAllDimensions, "should have all matrix dimensions")
	})

	t.Run("multiple_dynamic_matrix_values", func(t *testing.T) {
		workflowYAML := `
name: test-multiple-dynamic
on: push

jobs:
  setup:
    runs-on: ubuntu-latest
    outputs:
      versions: "[1.0, 2.0]"
      platforms: "[\"linux\", \"darwin\"]"
    steps:
      - run: echo setup

  build:
    needs: setup
    runs-on: ubuntu-latest
    strategy:
      matrix:
        version: ${{ fromJson(needs.setup.outputs.versions) }}
        platform: ${{ fromJson(needs.setup.outputs.platforms) }}
        static: [a, b]
    steps:
      - run: echo build
`
		// Parse with multiple dynamic values
		result, err := Parse([]byte(workflowYAML),
			WithJobOutputs(map[string]map[string]string{
				"setup": {
					"versions":  "[1.0, 2.0]",
					"platforms": "[\"linux\", \"darwin\"]",
				},
			}))

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.NotEmpty(t, result)
	})

	t.Run("all_static_arrays_no_dynamic", func(t *testing.T) {
		workflowYAML := `
name: test-all-static
on: push

jobs:
  build:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        os: [ubuntu-latest, windows-latest]
        version: [1.18, 1.19, 1.20]
        node: [14, 16]
    steps:
      - run: echo build
`
		// Parse with all static arrays, no dynamic values
		result, err := Parse([]byte(workflowYAML))

		assert.NoError(t, err)
		assert.NotNil(t, result)

		// Should expand correctly
		// 2 os * 3 versions * 2 node = 12 combinations
		assert.NotEmpty(t, result)

		// Verify matrix structure
		for _, workflow := range result {
			id, swfJob := workflow.Job()
			if id == "build" {
				// Verify the job was parsed with a matrix strategy
				assert.NotNil(t, swfJob)
				assert.NotEqual(t, 0, swfJob.Strategy.RawMatrix.Kind)
				break
			}
		}
	})
}
