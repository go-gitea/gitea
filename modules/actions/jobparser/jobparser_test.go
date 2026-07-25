// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jobparser

import (
	"fmt"
	"strings"
	"testing"

	"gitea.com/gitea/runner/act/model"
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
			name:    "prefixed_newline",
			options: nil,
			wantErr: false,
		},
		{
			name:    "continue_on_error_expr",
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

func TestParseDefersDynamicMatrix(t *testing.T) {
	// A matrix referencing needs outputs yields one placeholder keeping the raw expression, rather
	// than one job per resolvable static value. Any other matrix expands at plan time as usual.
	const workflow = `
on: push
jobs:
  setup:
    steps: [{run: echo}]
  build:
    needs: setup
    strategy:
      matrix:
        os: [a, b]
        version: %s
    steps: [{run: echo}]
`
	for _, tt := range []struct {
		name     string
		version  string
		deferred bool
		want     int
	}{
		{"needs outputs", "${{ fromJson(needs.setup.outputs.v) }}", true, 1},
		{"static", "[1, 2]", false, 4},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Parse(fmt.Appendf(nil, workflow, tt.version))
			require.NoError(t, err)

			var builds []*Job
			for _, w := range result {
				if id, job := w.Job(); id == "build" {
					builds = append(builds, job)
				}
			}
			require.Len(t, builds, tt.want)
			assert.Equal(t, tt.deferred, HasUnevaluatedMatrix(builds[0]))
		})
	}
}

func TestExpandMatrixWithNeeds(t *testing.T) {
	// matrixYAML is the YAML value of the `matrix:` key, so a case can replace the whole node.
	expand := func(t *testing.T, matrixYAML string) ([]*Job, error) {
		t.Helper()
		var strategy Strategy
		require.NoError(t, yaml.Unmarshal([]byte("matrix:"+matrixYAML), &strategy))
		job := &Job{Name: "build", Strategy: strategy}
		require.NoError(t, job.RawRunsOn.Encode("${{ matrix.os || 'ubuntu-latest' }}"))
		require.NoError(t, job.RawNeeds.Encode([]string{"setup"}))
		// The results map must describe the job itself too, as findJobNeedsAndFillJobResults does.
		return ExpandMatrixWithNeeds("build", job, &model.GithubContext{}, map[string]*JobResult{
			"build": {Needs: []string{"setup"}},
			"setup": {Result: "success", Outputs: map[string]string{
				"versions": `["1.20", "1.21"]`,
				"os":       `["linux", "darwin"]`,
				"include":  `[{"os":"linux","fast":true},{"os":"windows","fast":false}]`,
				"empty":    "[]",
			}},
		}, nil, nil)
	}

	t.Run("expands the product and interpolates runs-on", func(t *testing.T) {
		got, err := expand(t, "\n  os: ${{ fromJson(needs.setup.outputs.os) }}\n  version: ${{ fromJson(needs.setup.outputs.versions) }}\n")
		require.NoError(t, err)
		names := make([]string, 0, len(got))
		for _, combo := range got {
			names = append(names, combo.Name)
			assert.Contains(t, []string{"linux", "darwin"}, combo.RunsOn()[0])
		}
		// Dimensions are appended in key order, as GitHub names multi-dimension combinations.
		assert.ElementsMatch(t, []string{
			"build (linux, 1.20)", "build (linux, 1.21)", "build (darwin, 1.20)", "build (darwin, 1.21)",
		}, names)
	})

	t.Run("static and dynamic dimensions expand together, once", func(t *testing.T) {
		got, err := expand(t, "\n  os: [linux, darwin]\n  version: ${{ fromJson(needs.setup.outputs.versions) }}\n")
		require.NoError(t, err)
		assert.Len(t, got, 4)
	})

	t.Run("include-only matrix expands", func(t *testing.T) {
		got, err := expand(t, "\n  include: ${{ fromJson(needs.setup.outputs.include) }}\n")
		require.NoError(t, err)
		assert.Len(t, got, 2)
	})

	// GitHub rejects a matrix that yields no combinations instead of running the job unparameterized.
	for _, tt := range []struct{ name, matrix string }{
		{"empty vector", "\n  version: ${{ fromJson(needs.setup.outputs.empty) }}\n"},
		{"empty include", "\n  include: ${{ fromJson(needs.setup.outputs.empty) }}\n"},
		{"whole matrix not a mapping", " ${{ fromJson(needs.setup.outputs.empty) }}\n"},
	} {
		t.Run(tt.name+" errors", func(t *testing.T) {
			_, err := expand(t, tt.matrix)
			require.ErrorContains(t, err, "matrix must define at least one vector")
		})
	}

	t.Run("unresolved need errors", func(t *testing.T) {
		_, err := expand(t, "\n  v: ${{ fromJson(needs.missing.outputs.v) }}\n")
		require.ErrorContains(t, err, "evaluate matrix")
	})
}
