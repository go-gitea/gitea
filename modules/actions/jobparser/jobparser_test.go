// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jobparser

import (
	"fmt"
	"strings"
	"testing"

	"gitea.dev/actionslib/pkg/expreval"
	"gitea.dev/actionslib/pkg/exprparser"
	"gitea.dev/actionslib/pkg/model"

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
      %s
      matrix:
        os: [a, b]
        version: %s
    steps: [{run: echo}]
`
	for _, tt := range []struct {
		name     string
		needs    string
		strategy string
		version  string
		deferred bool
		want     int
	}{
		{"needs outputs", "needs: setup", "", "${{ fromJson(needs.setup.outputs.v) }}", true, 1},
		{"static", "needs: setup", "", "[1, 2]", false, 4},
		{"max-parallel over needs outputs", "needs: setup", "max-parallel: ${{ needs.setup.outputs.limit }}", "[1, 2]", true, 1},
		// Without needs there is nothing to resolve the expression from later, so deferring would
		// strand the job as a single combination that never expands.
		{"expression without needs", "", "", `["${{ github.sha }}"]`, false, 2},
		// A context that is already available while planning must keep expanding there, otherwise
		// such a workflow would silently lose the per-combination commit statuses it used to create.
		{"expression over another context", "needs: setup", "", `["${{ github.sha }}"]`, false, 2},
		// The needs context is looked up in the parsed expression, not in the raw text.
		{"needs inside a string literal", "needs: setup", "", `["${{ format('needs.setup.outputs.v {0}', github.sha) }}"]`, false, 2},
		{"vars", "needs: setup", "", "${{ fromJSON(vars.VERSIONS) }}", false, 4},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Parse(fmt.Appendf(nil, workflow, tt.needs, tt.strategy, tt.version), WithGitContext(&model.GithubContext{}), WithVars(map[string]string{"VERSIONS": "[1, 2]"}))
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

func TestParseWholeValueExpressions(t *testing.T) {
	const workflow = `
on: push
env: ${{ fromJSON(vars.ENV) }}
jobs:
  setup:
    runs-on: ubuntu-latest
    steps: [{run: echo}]
  build:
    %s
    runs-on: ${{ matrix.os }}
    strategy: ${{ fromJSON(%s) }}
    services: ${{ fromJSON(vars.SERVICES) }}
    defaults:
      run: ${{ fromJSON(vars.DEFAULTS) }}
    steps:
      - uses: actions/checkout@v4
        with: ${{ fromJSON(vars.WITH) }}
`
	const strategy = `{"max-parallel": 1, "matrix": {"os": ["a", "b-${{ secrets.X }}"]}}`
	runnerView := func(t *testing.T, job *model.Job) []any {
		t.Helper()
		runner := expreval.New(exprparser.NewInterpeter(&exprparser.EvaluationEnvironment{Secrets: map[string]string{"X": "leaked"}}, exprparser.Config{}).Evaluate)
		require.NoError(t, runner.EvaluateYamlNode(&job.Strategy.RawMatrix))
		require.NoError(t, runner.EvaluateYamlNode(&job.RawRunsOn))
		matrixes, err := job.GetMatrixes()
		require.NoError(t, err)
		name, err := runner.Interpolate(job.Name)
		require.NoError(t, err)
		return []any{matrixes[0]["os"], name, job.RunsOn()}
	}
	payloads := func(t *testing.T, needs, source string, options ...ParseOption) []string {
		t.Helper()
		workflows, err := Parse(fmt.Appendf(nil, workflow, needs, source), options...)
		require.NoError(t, err)
		var payloads []string
		for _, w := range workflows {
			if id, _ := w.Job(); id == "build" {
				payload, err := w.Marshal()
				require.NoError(t, err)
				payloads = append(payloads, string(payload))
			}
		}
		return payloads
	}
	planning := []ParseOption{WithGitContext(&model.GithubContext{}), WithVars(map[string]string{"STRATEGY": strategy})}

	planned := payloads(t, "", "vars.STRATEGY", planning...)
	require.Len(t, planned, 2)
	for _, raw := range []string{`max-parallel: "1"`, "env: ${{ fromJSON(vars.ENV) }}", "services: ${{ fromJSON(vars.SERVICES) }}", "run: ${{ fromJSON(vars.DEFAULTS) }}", "with: ${{ fromJSON(vars.WITH) }}"} {
		assert.Contains(t, planned[0], raw)
	}
	for index, payload := range planned {
		os := []string{"a", "b-${{ secrets.X }}"}[index]
		read, err := model.ReadWorkflow(strings.NewReader(payload))
		require.NoError(t, err)
		assert.Equal(t, []any{os, "build (" + os + ")", []string{os}}, runnerView(t, read.Jobs["build"]))
		_, job, err := ParseRawSingleWorkflow([]byte(payload))
		require.NoError(t, err)
		group, _, err := EvaluateConcurrency(&model.RawConcurrency{Group: "${{ matrix.os }}"}, "build", job, map[string]any{}, nil, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, []any{"build (" + os + ")", []string{os}, os}, []any{job.DisplayName(), job.RunsOn(), group})
		_, err = Parse([]byte(payload))
		require.NoError(t, err)
	}

	assert.Len(t, payloads(t, "", "vars.STRATEGY"), 1)

	deferred := payloads(t, "needs: setup", "needs.setup.outputs.strategy", planning...)
	require.Len(t, deferred, 1)
	_, placeholder, err := ParseRawSingleWorkflow([]byte(deferred[0]))
	require.NoError(t, err)
	expanded, err := ExpandMatrixWithNeeds("build", placeholder, &model.GithubContext{}, map[string]*JobResult{
		"build": {Needs: []string{"setup"}},
		"setup": {Result: "success", Outputs: map[string]string{"strategy": strategy}},
	}, nil, nil, 256)
	require.NoError(t, err)
	require.Len(t, expanded, 2)
	assert.Equal(t, "1", expanded[0].Strategy.MaxParallelString)
	assert.Equal(t, []any{"build (b-${{ secrets.X }})", []string{"b-${{ secrets.X }}"}}, []any{expanded[1].DisplayName(), expanded[1].RunsOn()})
	assert.Equal(t, []any{"b-${{ secrets.X }}", "build (b-${{ secrets.X }})", []string{"b-${{ secrets.X }}"}},
		runnerView(t, &model.Job{Name: expanded[1].Name, RawRunsOn: expanded[1].RawRunsOn, Strategy: expanded[1].Strategy.actStrategy()}))

	_, err = Parse(fmt.Appendf(nil, workflow, "", "vars.STRATEGY"), planning[0], WithVars(map[string]string{"STRATEGY": `"a"`}))
	require.ErrorContains(t, err, "is not a map of strategy keys to values")
}

func TestParseInterpolatesRunName(t *testing.T) {
	workflow := func(runName string) []byte {
		return []byte("name: t\nrun-name: \"" + runName + "\"\non: push\njobs:\n  build:\n    runs-on: ubuntu-latest\n    steps: [{run: echo}]\n")
	}

	for _, tt := range []struct{ name, runName, want string }{
		{"bool", "${{ true }}", "true"},
		{"int", "${{ 1 }}", "1"},
		{"float", "${{ 1.0 }}", "1"},
		{"null", "${{ null }}", ""},
		{"object", `${{ fromJSON('{\"a\":1}') }}`, "Object"},
		{"array", "${{ fromJSON('[1,2]') }}", "Array"},
		{"context", "${{ github }}", "Object"},
		{"surrounding literals", "run ${{ 1 }} now", "run 1 now"},
		{"two expressions", "${{ 1 }}-${{ true }}", "1-true"},
		{"closing brace inside a string", "${{ 'a}}b' }}", "a}}b"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Parse(workflow(tt.runName), WithGitContext(&model.GithubContext{EventName: "push"}))
			require.NoError(t, err)
			require.Len(t, result, 1)
			assert.Equal(t, tt.want, result[0].RunName)
		})
	}

	t.Run("unclosed expression errors", func(t *testing.T) {
		_, err := Parse(workflow("${{ 1"), WithGitContext(&model.GithubContext{EventName: "push"}))
		require.ErrorContains(t, err, "unclosed expression")
	})

	// a malformed part must not restructure the surrounding expression
	for _, runName := range []string{"${{ 1) && (2 }}", "run ${{ 1) && (2 }} now", "${{ 'a' }} ${{ b", "${{ 'a }}"} {
		_, err := Parse(workflow(runName), WithGitContext(&model.GithubContext{EventName: "push"}))
		assert.ErrorContains(t, err, "interpolate run-name")
	}

	// callers such as commit status parse without a git context, leaving `github` a nil pointer
	result, err := Parse(workflow("${{ github }}"))
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Empty(t, result[0].RunName)
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
				"limit":    "3",
			}},
		}, nil, nil, maxCombinations)
	}
	expand := func(t *testing.T, matrixYAML string) ([]*Job, error) {
		t.Helper()
		return expandMax(t, matrixYAML, 256)
	}

	t.Run("expands the product and interpolates runs-on", func(t *testing.T) {
		got, err := expand(t, "\n  os: ${{ fromJson(needs.setup.outputs.os) }}\n  version: ${{ fromJson(needs.setup.outputs.versions) }}\nmax-parallel: ${{ needs.setup.outputs.limit }}\n")
		require.NoError(t, err)
		indexes := make(map[string]int, len(got))
		for _, combo := range got {
			indexes[combo.Name] = combo.Strategy.JobIndex
			assert.Contains(t, []string{"linux", "darwin"}, combo.RunsOn()[0])
			assert.Equal(t, "3", combo.Strategy.MaxParallelString)
			assert.Equal(t, 4, combo.Strategy.JobTotal)
		}
		// Dimensions are appended in key order, as GitHub names multi-dimension combinations.
		assert.Equal(t, map[string]int{
			"build (linux, 1.20)": 0, "build (linux, 1.21)": 1, "build (darwin, 1.20)": 2, "build (darwin, 1.21)": 3,
		}, indexes)
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
	for _, tt := range []struct{ name, matrix, errHas string }{
		{"empty vector", "\n  version: ${{ fromJson(needs.setup.outputs.empty) }}\n", `Matrix vector "version" does not contain any values`},
		{"empty include", "\n  include: ${{ fromJson(needs.setup.outputs.empty) }}\n", "Matrix must define at least one vector"},
		{"whole matrix not a mapping", " ${{ fromJson(needs.setup.outputs.empty) }}\n", `Matrix "" is not a map of matrix keys to values`},
	} {
		t.Run(tt.name+" errors", func(t *testing.T) {
			_, err := expand(t, tt.matrix)
			require.ErrorContains(t, err, tt.errHas)
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
	for _, filter := range []string{"include", "exclude"} {
		t.Run(filter, func(t *testing.T) {
			_, err := Parse(fmt.Appendf(nil,
				"name: t\non: push\njobs:\n  build:\n    runs-on: ubuntu-latest\n    strategy:\n      matrix:\n        os: [a]\n        %s: ${{ fromJson(vars.MATRIX) }}\n    steps: [{run: echo}]\n", filter),
				WithGitContext(&model.GithubContext{}), WithVars(map[string]string{"MATRIX": `"a"`}))
			require.ErrorContains(t, err, "is not a list of maps")

			_, err = evaluateJobIf(t, fmt.Sprintf("os: [a]\n  %s: ${{ fromJson(vars.MATRIX) }}", filter), "${{ true }}", false)
			require.ErrorContains(t, err, "is not a list of maps")
		})
	}
}

func TestParseRawSingleWorkflowRoundTripsDeferredPlaceholder(t *testing.T) {
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
	for name, matrix := range map[string]string{
		"include expression":           "include: ${{ fromJson(needs.setup.outputs.m) }}",
		"static vector and expression": "os: [a, b]\n        version: ${{ fromJson(needs.setup.outputs.m) }}",
		"single expression vector":     "version: ${{ fromJson(needs.setup.outputs.m) }}",
	} {
		t.Run(name, func(t *testing.T) {
			planned, err := Parse(fmt.Appendf(nil, workflow, matrix))
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

			_, job, err := ParseRawSingleWorkflow(payload)
			require.NoError(t, err)
			assert.Equal(t, "build", job.Name)
			assert.Empty(t, job.Needs())
			assert.False(t, HasDeferredMatrix(job))

			reparsed, err := Parse(payload)
			require.NoError(t, err)
			assert.Len(t, reparsed, 1)
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
