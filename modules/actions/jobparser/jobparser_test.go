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
    %s
    strategy:
      matrix:
        os: [a, b]
        version: %s
    steps: [{run: echo}]
`
	for _, tt := range []struct {
		name     string
		needs    string
		version  string
		deferred bool
		want     int
	}{
		{"needs outputs", "needs: setup", "${{ fromJson(needs.setup.outputs.v) }}", true, 1},
		{"static", "needs: setup", "[1, 2]", false, 4},
		// Without needs there is nothing to resolve the expression from later, so deferring would
		// strand the job as a single combination that never expands.
		{"expression without needs", "", `["${{ github.sha }}"]`, false, 2},
		// A context that is already available while planning must keep expanding there, otherwise
		// such a workflow would silently lose the per-combination commit statuses it used to create.
		{"expression over another context", "needs: setup", `["${{ github.sha }}"]`, false, 2},
		// The needs context is looked up in the parsed expression, not in the raw text.
		{"needs inside a string literal", "needs: setup", `["${{ format('needs.setup.outputs.v {0}', github.sha) }}"]`, false, 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Parse(fmt.Appendf(nil, workflow, tt.needs, tt.version))
			require.NoError(t, err)

			var builds []*Job
			for _, w := range result {
				if id, job := w.Job(); id == "build" {
					builds = append(builds, job)
				}
			}
			require.Len(t, builds, tt.want)
			assert.Equal(t, tt.deferred, HasDeferredMatrix(builds[0]))
		})
	}
}

func TestExpandMatrixWithNeeds(t *testing.T) {
	// matrixYAML is the YAML value of the `matrix:` key, so a case can replace the whole node.
	expandMax := func(t *testing.T, matrixYAML string, maxCombinations int) ([]*Job, error) {
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
		}, nil, nil, maxCombinations)
	}
	expand := func(t *testing.T, matrixYAML string) ([]*Job, error) {
		t.Helper()
		return expandMax(t, matrixYAML, 256)
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

	// The combination count comes from a runtime output, so it must be rejected before one Job per
	// combination is built rather than after.
	t.Run("too many combinations errors", func(t *testing.T) {
		_, err := expandMax(t, "\n  version: ${{ fromJson(needs.setup.outputs.versions) }}\n", 1)
		require.ErrorContains(t, err, "exceeding the limit of 1")
	})
}

// evaluateJobIf builds a one-job workflow around the given `matrix:` value and `if:`, and decides it.
func evaluateJobIf(t *testing.T, matrixYAML, ifExpr string, deferred bool) (bool, error) {
	t.Helper()
	var strategy Strategy
	require.NoError(t, yaml.Unmarshal(fmt.Appendf(nil, "matrix:\n  %s\n", matrixYAML), &strategy))
	job := &Job{Name: "build", Strategy: strategy}
	require.NoError(t, job.If.Encode(ifExpr))
	return EvaluateJobIfExpression("build", job, map[string]any{}, map[string]*JobResult{"build": {}}, nil, nil, deferred)
}

func TestRejectsUnevaluatedMatrixFilters(t *testing.T) {
	// act dereferences include/exclude entries as mappings without checking, so an unevaluated
	// expression panics there. Every entry point into act's matrix expansion must reject it. The
	// expression here reads `vars`, which is available while planning, so the job is not deferred and
	// nothing will ever resolve the filter: the error is the right answer at both entry points.
	// A deferred placeholder is the other case, covered by TestEvaluateJobIfExpressionLeavesRawMatrixUnavailable.
	for _, filter := range []string{"include", "exclude"} {
		t.Run(filter, func(t *testing.T) {
			_, err := Parse(fmt.Appendf(nil,
				"name: t\non: push\njobs:\n  build:\n    runs-on: ubuntu-latest\n    strategy:\n      matrix:\n        os: [a]\n        %s: ${{ fromJson(vars.MATRIX) }}\n    steps: [{run: echo}]\n", filter))
			require.ErrorContains(t, err, "must be a list of mappings")

			_, err = evaluateJobIf(t, fmt.Sprintf("os: [a]\n  %s: ${{ fromJson(vars.MATRIX) }}", filter), "${{ true }}", false)
			require.ErrorContains(t, err, "must be a list of mappings")
		})
	}
}

func TestParseRawSingleWorkflowRoundTripsDeferredPlaceholder(t *testing.T) {
	// The server persists a placeholder the way insertRunJob does: erase the needs, then marshal.
	// Reading it back must yield that one job again. Parse cannot do it: it only keeps a matrix raw
	// while the job still declares needs, so on the stored payload it falls through to expanding the
	// raw matrix instead - which either fails or splits the placeholder into several workflows, and
	// in both cases leaves the job unexpandable for good.
	const workflow = `
on: push
jobs:
  setup:
    steps: [{run: echo}]
  build:
    needs: setup
    runs-on: ubuntu-latest
    strategy:
      matrix:
        %s
    steps: [{run: echo}]
`
	for _, tt := range []struct {
		name        string
		matrix      string
		parseCount  int    // what Parse makes of the stored payload
		parseErrHas string // ... or the error it fails with
	}{
		// The canonical GitHub dynamic-matrix idiom. act dereferences include entries as mappings, so
		// validateMatrixFilters rejects the still-scalar expression outright.
		{name: "include expression", matrix: "include: ${{ fromJson(needs.setup.outputs.m) }}", parseErrHas: "must be a list of mappings"},
		// A static vector crossed with the unevaluated expression: one workflow per static value.
		{name: "static vector and expression", matrix: "os: [a, b]\n        version: ${{ fromJson(needs.setup.outputs.m) }}", parseCount: 2},
		// The single-key case the feature shipped with happens to survive Parse, so it must keep working.
		{name: "single expression vector", matrix: "version: ${{ fromJson(needs.setup.outputs.m) }}", parseCount: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			planned, err := Parse(fmt.Appendf(nil, workflow, tt.matrix))
			require.NoError(t, err)

			var payload []byte
			for _, w := range planned {
				id, job := w.Job()
				if id != "build" {
					continue
				}
				require.True(t, HasDeferredMatrix(job), "build must be planned as a placeholder")
				require.NoError(t, w.SetJob(id, job.EraseNeeds()))
				payload, err = w.Marshal()
				require.NoError(t, err)
			}
			require.NotEmpty(t, payload, "no placeholder was planned for build")

			// The stored payload keeps the raw matrix, but no longer the needs that made Parse defer it.
			_, job, err := ParseRawSingleWorkflow(payload)
			require.NoError(t, err)
			assert.Equal(t, "build", job.Name)
			// The needs are gone, which is exactly why Parse no longer defers this payload.
			assert.Empty(t, job.Needs())
			assert.False(t, HasDeferredMatrix(job))

			// Guard the reason ParseRawSingleWorkflow exists, so a future Parse change cannot quietly
			// make the placeholder re-expandable again without this being noticed.
			reparsed, err := Parse(payload)
			if tt.parseErrHas != "" {
				require.ErrorContains(t, err, tt.parseErrHas)
			} else {
				require.NoError(t, err)
				assert.Len(t, reparsed, tt.parseCount)
			}
		})
	}
}

func TestEvaluateJobIfExpressionLeavesRawMatrixUnavailable(t *testing.T) {
	// A placeholder's `if:` is read before its matrix can be resolved. `matrix.*` has to be absent
	// there: binding it to the expression's own source text would decide the job against a value no
	// combination ever has, and an include/exclude that is still a scalar cannot be read at all.
	t.Run("include expression is not read", func(t *testing.T) {
		run, err := evaluateJobIf(t, "include: ${{ fromJson(needs.setup.outputs.m) }}", "${{ true }}", true)
		require.NoError(t, err)
		assert.True(t, run)
	})

	t.Run("matrix context is null, not the raw expression", func(t *testing.T) {
		const matrix = "version: ${{ fromJson(needs.setup.outputs.m) }}"
		run, err := evaluateJobIf(t, matrix, "${{ matrix.version == null }}", true)
		require.NoError(t, err)
		assert.True(t, run)

		run, err = evaluateJobIf(t, matrix, "${{ matrix.version == '${{ fromJson(needs.setup.outputs.m) }}' }}", true)
		require.NoError(t, err)
		assert.False(t, run)
	})

	t.Run("an expanded job still reads its combination", func(t *testing.T) {
		run, err := evaluateJobIf(t, "version: [1]", "${{ matrix.version == 1 }}", false)
		require.NoError(t, err)
		assert.True(t, run)
	})
}

func TestExpressionReadsMatrix(t *testing.T) {
	// Erring toward true only postpones the `if:` to the pass that has the combination, which decides it correctly anyway.
	for value, want := range map[string]bool{
		"":                                  false,
		"true":                              false, // a bare literal is an expression too, it just reads nothing
		"${{ always() }}":                   false,
		"${{ needs.setup.result == 'ok' }}": false,
		"${{ vars.MATRIX }}":                false, // a name that merely looks like the context
		"${{ matrix.os }}":                  true,
		"${{ MATRIX.os }}":                  true, // contexts are case-insensitive
		"${{ always() && matrix.os == 1 }}": true,
		"${{ contains(matrix.tags, 'a') }}": true,
		"${{ toJSON(matrix) }}":             true, // the whole context, not a property of it
		"${{ vars.A }}${{ matrix.os }}":     true, // only the second of two expressions reads it
		"${{ matrix.os == }}":               true, // unparseable, postpone rather than decide it here
		// An `if:` may omit the `${{ }}`, and is evaluated as one expression either way.
		"matrix.os == 'a'":           true,
		"needs.setup.result == 'ok'": false,
	} {
		assert.Equal(t, want, ExpressionReadsMatrix(value), "value %q", value)
	}
}

func TestExpressionIgnoresNeedResults(t *testing.T) {
	for value, want := range map[string]bool{
		"":                             false,
		"${{ matrix.os == 'a' }}":      false,
		"${{ success() }}":             false, // the implicit gate, so the fallback already matches it
		"${{ always() }}":              true,
		"${{ ALWAYS() && matrix.os }}": true, // function names are case-insensitive
		"${{ failure() }}":             true,
		"${{ cancelled() }}":           true,
		"always() && matrix.os == 'a'": true, // the brace-less form of the same gate
		"${{ vars.always }}":           false,
	} {
		assert.Equal(t, want, ExpressionIgnoresNeedResults(value), "value %q", value)
	}
}
