// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jobparser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"
)

func TestMatrixDependsOnNeedsOutputs(t *testing.T) {
	t.Run("zero node returns false", func(t *testing.T) {
		assert.False(t, MatrixDependsOnNeedsOutputs(yaml.Node{}))
	})

	cases := []struct {
		name   string
		input  string
		expect bool
	}{
		{"static matrix", "os:\n  - linux\n  - windows\n", false},
		{"matrix references inputs only", "thing: ${{ inputs.what }}\n", false},
		{"matrix references vars", "thing: ${{ vars.LIST }}\n", false},
		{"matrix references needs.result (not outputs)", "thing: ${{ needs.job1.result }}\n", false},
		{"matrix references needs.outputs directly", "manifest: ${{ fromJSON(needs.job1.outputs.matrix) }}\n", true},
		{"matrix references needs.outputs nested", "key: ${{ needs.upstream-job.outputs.list }}\n", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var node yaml.Node
			require.NoError(t, yaml.Unmarshal([]byte(c.input), &node))
			// yaml.Unmarshal produces a DocumentNode wrapping the actual content;
			// MatrixDependsOnNeedsOutputs works on either since Marshal round-trips
			// the original text. Use the content node for fidelity with the actual
			// callers, which read job.Strategy.RawMatrix (mapping/scalar nodes).
			require.NotEmpty(t, node.Content)
			assert.Equal(t, c.expect, MatrixDependsOnNeedsOutputs(*node.Content[0]))
		})
	}
}

const deferredMatrixWorkflow = `name: deferred-matrix
on: push
jobs:
  prepare:
    runs-on: ubuntu-latest
    outputs:
      manifest: ${{ steps.set.outputs.manifest }}
    steps:
      - id: set
        run: echo "manifest=[\"a\",\"b\"]" >> $GITHUB_OUTPUT
  build:
    needs: prepare
    runs-on: ubuntu-latest
    strategy:
      matrix:
        manifest: ${{ fromJSON(needs.prepare.outputs.manifest) }}
    steps:
      - run: echo ${{ matrix.manifest }}
`

func TestParseEmitsPlaceholderForDeferredMatrix(t *testing.T) {
	workflows, err := Parse([]byte(deferredMatrixWorkflow))
	require.NoError(t, err)
	// One SingleWorkflow for `prepare` (no matrix), one PLACEHOLDER for `build`.
	require.Len(t, workflows, 2)

	var build *SingleWorkflow
	for _, w := range workflows {
		id, _ := w.Job()
		if id == "build" {
			build = w
		}
	}
	require.NotNil(t, build, "build job must be emitted as a placeholder")

	_, buildJob := build.Job()
	require.NotNil(t, buildJob)
	// Placeholder preserves needs (the job emitter uses them later).
	assert.Equal(t, []string{"prepare"}, buildJob.Needs())
	// Placeholder retains the raw matrix expression for later evaluation.
	assert.True(t, MatrixDependsOnNeedsOutputs(buildJob.Strategy.RawMatrix))
}

func TestExpandDeferredMatrix(t *testing.T) {
	parsed, err := Parse([]byte(deferredMatrixWorkflow))
	require.NoError(t, err)

	var placeholder *SingleWorkflow
	for _, w := range parsed {
		id, _ := w.Job()
		if id == "build" {
			placeholder = w
		}
	}
	require.NotNil(t, placeholder)
	payload, err := placeholder.Marshal()
	require.NoError(t, err)

	outputs := map[string]map[string]string{
		"prepare": {"manifest": `["a","b"]`},
	}
	expanded, err := ExpandDeferredMatrix(payload, []string{"prepare"}, outputs)
	require.NoError(t, err)
	require.Len(t, expanded, 2, "matrix produced two iterations")

	got := make([]string, 0, len(expanded))
	for _, w := range expanded {
		_, job := w.Job()
		require.NotNil(t, job)
		// Each expanded iteration's matrix is pinned to a single value.
		var pinned map[string][]any
		require.NoError(t, job.Strategy.RawMatrix.Decode(&pinned))
		require.Len(t, pinned["manifest"], 1)
		got = append(got, pinned["manifest"][0].(string))
	}
	assert.ElementsMatch(t, []string{"a", "b"}, got)
}

func TestExpandDeferredMatrixRequiresSingleJob(t *testing.T) {
	// A workflow with two jobs in the payload should be rejected — the
	// emitter only calls ExpandDeferredMatrix on a placeholder, which is
	// always a single-job SingleWorkflow.
	multiJob := []byte("name: x\non: push\njobs:\n  a:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo a\n  b:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo b\n")
	_, err := ExpandDeferredMatrix(multiJob, nil, nil)
	require.Error(t, err)
}
