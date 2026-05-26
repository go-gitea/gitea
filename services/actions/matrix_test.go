// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"
)

func TestHasMatrixWithNeeds(t *testing.T) {
	tests := []struct {
		name     string
		strategy string
		want     bool
	}{
		{
			name: "dynamic matrix referencing job output",
			strategy: `
matrix:
  version: ${{ fromJson(needs.generate.outputs.matrix) }}
`,
			want: true,
		},
		{
			name: "static matrix — no expression",
			strategy: `
matrix:
  os: [ubuntu-latest, windows-latest]
`,
			want: false,
		},
		{
			name: "value contains needs. but not inside expression",
			strategy: `
matrix:
  os: [needs.review-runner]
`,
			want: false,
		},
		{
			name: "needs. outside expression block",
			strategy: `
matrix:
  runner: needs.something-but-no-braces
`,
			want: false,
		},
		{
			name: "expression with needs but no .outputs.",
			strategy: `
matrix:
  version: ${{ needs.job1 }}
`,
			want: false,
		},
		{
			name:     "empty strategy",
			strategy: "",
			want:     false,
		},
		{
			name: "strategy without matrix key",
			strategy: `
fail-fast: false
`,
			want: false,
		},
		{
			name: "multi-dimension dynamic matrix",
			strategy: `
matrix:
  os: [ubuntu-latest, windows-latest]
  version: ${{ fromJson(needs.setup.outputs.versions) }}
`,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, HasMatrixWithNeeds(tt.strategy))
		})
	}
}

func TestMergeNeedsIntoVars(t *testing.T) {
	base := map[string]string{"MY_VAR": "hello"}
	needs := map[string]*TaskNeed{
		"setup": {
			Result:  actions_model.StatusSuccess,
			Outputs: map[string]string{"versions": `["1","2"]`, "extra": "val"},
		},
	}
	merged := mergeNeedsIntoVars(base, needs)

	assert.Equal(t, "hello", merged["MY_VAR"])
	assert.Equal(t, `["1","2"]`, merged["needs.setup.outputs.versions"])
	assert.Equal(t, "val", merged["needs.setup.outputs.extra"])
	// base must not be mutated
	assert.NotContains(t, base, "needs.setup.outputs.versions")
}

func TestConstructWorkflowWithNeeds(t *testing.T) {
	// Minimal WorkflowPayload with a strategy referencing a needs output.
	payload, err := yaml.Marshal(map[string]any{
		"jobs": map[string]any{
			"build": map[string]any{
				"runs-on": "ubuntu-latest",
				"strategy": map[string]any{
					"matrix": map[string]any{
						"version": `${{ fromJson(needs.setup.outputs.versions) }}`,
					},
				},
				"steps": []any{},
			},
		},
	})
	require.NoError(t, err)

	job := &actions_model.ActionRunJob{
		JobID:           "build",
		WorkflowPayload: payload,
		RawStrategy: `
matrix:
  version: ${{ fromJson(needs.setup.outputs.versions) }}
`,
		Needs: []string{"setup"},
	}
	needs := map[string]*TaskNeed{
		"setup": {
			Result:  actions_model.StatusSuccess,
			Outputs: map[string]string{"versions": `["1.20","1.21"]`},
		},
	}

	out, err := constructWorkflowWithNeeds(job, needs)
	require.NoError(t, err)

	// The resulting YAML should contain both the build job and a stub for setup.
	var wf map[string]any
	require.NoError(t, yaml.Unmarshal(out, &wf))

	jobs, ok := wf["jobs"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, jobs, "build")
	assert.Contains(t, jobs, "setup", "stub for needs dependency must be present")

	// build job needs must list the dependency
	buildJob, ok := jobs["build"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, buildJob, "needs")

	// RawStrategy must be re-injected (not the pre-baked array form)
	strategy, ok := buildJob["strategy"].(map[string]any)
	require.True(t, ok)
	matrix, ok := strategy["matrix"].(map[string]any)
	require.True(t, ok)
	versionExpr, ok := matrix["version"].(string)
	require.True(t, ok)
	assert.Contains(t, versionExpr, "fromJson")
}

func TestConstructWorkflowWithNeeds_InvalidPayload(t *testing.T) {
	job := &actions_model.ActionRunJob{
		JobID:           "build",
		WorkflowPayload: []byte("not: valid: yaml: ["),
	}
	_, err := constructWorkflowWithNeeds(job, nil)
	assert.Error(t, err)
}

func TestConstructWorkflowWithNeeds_MissingJobsSection(t *testing.T) {
	payload, err := yaml.Marshal(map[string]any{"name": "test"})
	require.NoError(t, err)

	job := &actions_model.ActionRunJob{
		JobID:           "build",
		WorkflowPayload: payload,
	}
	_, err = constructWorkflowWithNeeds(job, nil)
	assert.Error(t, err)
}
