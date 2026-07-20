// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jobparser

import (
	"maps"
	"testing"

	"gitea.com/gitea/runner/act/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"
)

func TestParseWorkflowCallSpec(t *testing.T) {
	t.Run("malformed YAML surfaces a parse error", func(t *testing.T) {
		// Mismatched flow-sequence brackets — yaml.Unmarshal must reject this.
		_, err := ParseWorkflowCallSpec([]byte(`name: bad
on: [workflow_call
jobs:
  noop: { }
`))
		require.Error(t, err)
	})

	t.Run("workflow without on.workflow_call is rejected", func(t *testing.T) {
		notCallable := []byte(`name: ordinary
on: push
jobs:
  noop:
    runs-on: ubuntu-latest
    steps:
      - run: echo
`)
		_, err := ParseWorkflowCallSpec(notCallable)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not declare on.workflow_call")
	})

	t.Run("input missing the required type field is rejected", func(t *testing.T) {
		content := callableWorkflow(t, `inputs:
      x:
        description: missing type
`)
		_, err := ParseWorkflowCallSpec(content)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `missing required field "type"`)
	})

	t.Run("inputs/secrets/outputs are decoded", func(t *testing.T) {
		content := callableWorkflow(t, `inputs:
      env:
        type: string
        required: true
    secrets:
      DEPLOY_KEY:
        required: true
    outputs:
      sha:
        value: ${{ jobs.build.outputs.commit }}
`)
		spec, err := ParseWorkflowCallSpec(content)
		require.NoError(t, err)
		assert.Equal(t, InputTypeString, spec.Inputs["env"].Type)
		assert.True(t, spec.Inputs["env"].Required)
		assert.True(t, spec.Secrets["DEPLOY_KEY"].Required)
		assert.Equal(t, "${{ jobs.build.outputs.commit }}", spec.Outputs["sha"].Value)
	})
}

func TestEvaluateCallerWith(t *testing.T) {
	t.Run("empty with: returns empty map", func(t *testing.T) {
		out, err := EvaluateCallerWith("caller", &Job{}, nil, callerResults("caller", nil, nil), nil, nil)
		require.NoError(t, err)
		assert.Empty(t, out)
	})

	t.Run("non-string raw values pass through unchanged", func(t *testing.T) {
		job := &Job{With: map[string]any{
			"already_bool":  true,
			"already_int":   42,
			"already_slice": []any{"a", "b"},
		}}
		out, err := EvaluateCallerWith("caller", job, nil, callerResults("caller", nil, nil), nil, nil)
		require.NoError(t, err)
		assert.Equal(t, true, out["already_bool"])
		assert.Equal(t, 42, out["already_int"])
		assert.Equal(t, []any{"a", "b"}, out["already_slice"])
	})

	t.Run("expressions resolve against vars/inputs/results", func(t *testing.T) {
		job := &Job{With: map[string]any{
			"env_name":    "${{ vars.ENV }}",
			"from_inputs": "${{ inputs.PARENT_VAR }}",
			"from_needs":  "${{ needs.upstream.outputs.commit }}",
		}}
		gitCtx := map[string]any{"event": map[string]any{}}
		results := callerResults("caller", []string{"upstream"}, map[string]*JobResult{
			"upstream": {Result: "success", Outputs: map[string]string{"commit": "abc123"}},
		})
		vars := map[string]string{"ENV": "staging"}
		inputs := map[string]any{"PARENT_VAR": "from-parent"}
		out, err := EvaluateCallerWith("caller", job, gitCtx, results, vars, inputs)
		require.NoError(t, err)
		assert.Equal(t, "staging", out["env_name"])
		assert.Equal(t, "from-parent", out["from_inputs"])
		assert.Equal(t, "abc123", out["from_needs"])
	})

	t.Run("matrix.X resolves to this caller row's matrix instance", func(t *testing.T) {
		var rawMatrix yaml.Node
		require.NoError(t, rawMatrix.Encode(map[string][]any{"target": {"staging"}}))
		job := &Job{
			With:     map[string]any{"env": "${{ matrix.target }}"},
			Strategy: Strategy{RawMatrix: rawMatrix},
		}
		out, err := EvaluateCallerWith("caller", job, nil, callerResults("caller", nil, nil), nil, nil)
		require.NoError(t, err)
		assert.Equal(t, "staging", out["env"])
	})
}

func TestMatchCallerInputsAgainstSpec(t *testing.T) {
	// mustParseSpec wraps ParseWorkflowCallSpec for test brevity.
	mustParseSpec := func(t *testing.T, content []byte) *WorkflowCallSpec {
		t.Helper()
		spec, err := ParseWorkflowCallSpec(content)
		require.NoError(t, err)
		return spec
	}

	t.Run("default is filled when caller does not provide the input", func(t *testing.T) {
		spec := mustParseSpec(t, callableWorkflow(t, `inputs:
      greeting:
        type: string
        default: hi
`))
		out, err := MatchCallerInputsAgainstSpec(spec, nil)
		require.NoError(t, err)
		assert.Equal(t, map[string]any{"greeting": "hi"}, out)
	})

	t.Run("caller-provided value wins over default", func(t *testing.T) {
		spec := mustParseSpec(t, callableWorkflow(t, `inputs:
      greeting:
        type: string
        default: hi
`))
		out, err := MatchCallerInputsAgainstSpec(spec, map[string]any{"greeting": "hello"})
		require.NoError(t, err)
		assert.Equal(t, map[string]any{"greeting": "hello"}, out)
	})

	t.Run("required input must be provided", func(t *testing.T) {
		spec := mustParseSpec(t, callableWorkflow(t, `inputs:
      target:
        type: string
        required: true
`))
		_, err := MatchCallerInputsAgainstSpec(spec, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `"target" is required`)
	})

	t.Run("required input is satisfied by a default value", func(t *testing.T) {
		spec := mustParseSpec(t, callableWorkflow(t, `inputs:
      target:
        type: string
        required: true
        default: prod
`))
		out, err := MatchCallerInputsAgainstSpec(spec, nil)
		require.NoError(t, err)
		assert.Equal(t, map[string]any{"target": "prod"}, out)
	})

	t.Run("boolean inputs accept native bool values and bool defaults", func(t *testing.T) {
		spec := mustParseSpec(t, callableWorkflow(t, `inputs:
      flag1:
        type: boolean
      flag2:
        type: boolean
        default: true
      flag3:
        type: boolean
`))
		out, err := MatchCallerInputsAgainstSpec(spec, map[string]any{
			"flag1": true,
			"flag3": false,
		})
		require.NoError(t, err)
		assert.Equal(t, true, out["flag1"])
		assert.Equal(t, true, out["flag2"]) // from default
		assert.Equal(t, false, out["flag3"])
	})

	t.Run("boolean input rejects strings", func(t *testing.T) {
		spec := mustParseSpec(t, callableWorkflow(t, `inputs:
      flag:
        type: boolean
`))
		_, err := MatchCallerInputsAgainstSpec(spec, map[string]any{"flag": "true"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expects boolean")
	})

	t.Run("number inputs accept native numeric values and number defaults", func(t *testing.T) {
		spec := mustParseSpec(t, callableWorkflow(t, `inputs:
      count:
        type: number
      ratio:
        type: number
        default: 0.5
`))
		out, err := MatchCallerInputsAgainstSpec(spec, map[string]any{"count": 42})
		require.NoError(t, err)
		assert.InDelta(t, 42.0, out["count"], 0)
		assert.InDelta(t, 0.5, out["ratio"], 0)
	})

	t.Run("number input rejects strings", func(t *testing.T) {
		spec := mustParseSpec(t, callableWorkflow(t, `inputs:
      count:
        type: number
`))
		_, err := MatchCallerInputsAgainstSpec(spec, map[string]any{"count": "42"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expects number")
	})

	t.Run("unknown caller-with key is silently dropped", func(t *testing.T) {
		spec := mustParseSpec(t, callableWorkflow(t, `inputs:
      known:
        type: string
        default: ok
`))
		out, err := MatchCallerInputsAgainstSpec(spec, map[string]any{
			"known":   "yes",
			"unknown": "ignored",
		})
		require.NoError(t, err)
		assert.Equal(t, map[string]any{"known": "yes"}, out)
	})
}

func TestParseCallerSecrets(t *testing.T) {
	// secretYAMLNode unmarshals raw YAML text into a yaml.Node so tests can hand it to ParseCallerSecrets.
	secretYAMLNode := func(t *testing.T, s string) yaml.Node {
		t.Helper()
		var node yaml.Node
		require.NoError(t, yaml.Unmarshal([]byte(s), &node))
		// yaml.Unmarshal wraps content in a DocumentNode; the meaningful node is the first child.
		if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
			return *node.Content[0]
		}
		return node
	}

	t.Run("zero node returns no inherit, no mapping", func(t *testing.T) {
		inherit, mapping, err := ParseCallerSecrets(yaml.Node{})
		require.NoError(t, err)
		assert.False(t, inherit)
		assert.Nil(t, mapping)
	})

	t.Run("\"inherit\" scalar sets inherit=true", func(t *testing.T) {
		inherit, mapping, err := ParseCallerSecrets(secretYAMLNode(t, `inherit`))
		require.NoError(t, err)
		assert.True(t, inherit)
		assert.Nil(t, mapping)
	})

	t.Run("non-inherit scalar is rejected", func(t *testing.T) {
		_, _, err := ParseCallerSecrets(secretYAMLNode(t, `something-else`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected mapping or 'inherit'")
	})

	t.Run("mapping of secrets-style references is parsed", func(t *testing.T) {
		inherit, mapping, err := ParseCallerSecrets(secretYAMLNode(t, `
DEPLOY_KEY: ${{ secrets.GITEA_DEPLOY_KEY }}
DB_PASS:    ${{ secrets.PROD_DB_PASS }}
`))
		require.NoError(t, err)
		assert.False(t, inherit)
		assert.Equal(t, map[string]string{
			"DEPLOY_KEY": "GITEA_DEPLOY_KEY",
			"DB_PASS":    "PROD_DB_PASS",
		}, mapping)
	})

	t.Run("alias and source names are upper-cased", func(t *testing.T) {
		inherit, mapping, err := ParseCallerSecrets(secretYAMLNode(t, `
deploy_key: ${{ secrets.gitea_deploy_key }}
`))
		require.NoError(t, err)
		assert.False(t, inherit)
		assert.Equal(t, map[string]string{"DEPLOY_KEY": "GITEA_DEPLOY_KEY"}, mapping)
	})

	t.Run("mapping value not in ${{ secrets.NAME }} form is rejected", func(t *testing.T) {
		// plain string
		_, _, err := ParseCallerSecrets(secretYAMLNode(t, `KEY: not-an-expression`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), `must be of the form ${{ secrets.NAME }}`)

		// expression but referencing the wrong context (vars instead of secrets)
		_, _, err = ParseCallerSecrets(secretYAMLNode(t, `KEY: ${{ vars.NAME }}`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), `must be of the form ${{ secrets.NAME }}`)
	})
}

func TestValidateCallerSecrets(t *testing.T) {
	specWith := func(secrets map[string]SecretSpec) *WorkflowCallSpec {
		return &WorkflowCallSpec{Secrets: secrets}
	}

	t.Run("explicit mapping with all required + only declared aliases is accepted", func(t *testing.T) {
		spec := specWith(map[string]SecretSpec{
			"DEPLOY_KEY": {Required: true},
			"OPTIONAL":   {},
		})
		mapping := map[string]string{
			"DEPLOY_KEY": "PROD_DEPLOY_KEY",
			"OPTIONAL":   "SOMETHING_ELSE",
		}
		require.NoError(t, ValidateCallerSecrets(spec, mapping))
	})

	t.Run("alias not in callee schema is rejected", func(t *testing.T) {
		spec := specWith(map[string]SecretSpec{"DEPLOY_KEY": {}})
		mapping := map[string]string{
			"DEPLOY_KEY": "PROD_DEPLOY_KEY",
			"EXTRA":      "SOMETHING_NOT_DECLARED",
		}
		err := ValidateCallerSecrets(spec, mapping)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `caller secret "EXTRA"`)
		assert.Contains(t, err.Error(), `not declared`)
	})

	t.Run("missing required secret is rejected", func(t *testing.T) {
		spec := specWith(map[string]SecretSpec{
			"MUST_HAVE": {Required: true},
			"OPTIONAL":  {},
		})
		mapping := map[string]string{"OPTIONAL": "X"}
		err := ValidateCallerSecrets(spec, mapping)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `required secret "MUST_HAVE"`)
		assert.Contains(t, err.Error(), `not provided`)
	})

	t.Run("callee with no secrets schema accepts an empty mapping", func(t *testing.T) {
		spec := specWith(map[string]SecretSpec{})
		require.NoError(t, ValidateCallerSecrets(spec, nil))
		require.NoError(t, ValidateCallerSecrets(spec, map[string]string{}))
	})

	t.Run("callee with no secrets schema rejects a non-empty mapping", func(t *testing.T) {
		spec := specWith(map[string]SecretSpec{})
		err := ValidateCallerSecrets(spec, map[string]string{"X": "Y"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), `caller secret "X"`)
	})

	t.Run("name matching is case-insensitive", func(t *testing.T) {
		// declared name and caller alias differ only in case; both should match.
		spec := specWith(map[string]SecretSpec{"deploy_key": {Required: true}})
		mapping := map[string]string{"DEPLOY_KEY": "PROD_DEPLOY_KEY"}
		require.NoError(t, ValidateCallerSecrets(spec, mapping))
	})

	t.Run("nil spec is rejected", func(t *testing.T) {
		err := ValidateCallerSecrets(nil, map[string]string{"X": "Y"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nil workflow_call spec")
	})
}

func TestEvaluateWorkflowCallOutputs(t *testing.T) {
	t.Run("nil spec returns empty map", func(t *testing.T) {
		out, err := EvaluateWorkflowCallOutputs(nil, &model.GithubContext{}, nil, nil, nil)
		require.NoError(t, err)
		assert.Empty(t, out)
	})

	t.Run("spec with no outputs returns empty map", func(t *testing.T) {
		spec := &WorkflowCallSpec{Outputs: map[string]OutputSpec{}}
		out, err := EvaluateWorkflowCallOutputs(spec, &model.GithubContext{}, nil, nil, nil)
		require.NoError(t, err)
		assert.Empty(t, out)
	})

	t.Run("plain string value passes through unchanged", func(t *testing.T) {
		spec := &WorkflowCallSpec{Outputs: map[string]OutputSpec{
			"name": {Value: "static-value"},
		}}
		out, err := EvaluateWorkflowCallOutputs(spec, &model.GithubContext{}, nil, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"name": "static-value"}, out)
	})

	t.Run("output references jobs.<id>.outputs.<name>", func(t *testing.T) {
		spec := &WorkflowCallSpec{Outputs: map[string]OutputSpec{
			"sha": {Value: "${{ jobs.build.outputs.commit }}"},
		}}
		jobOutputs := JobOutputs{
			"build": {"commit": "deadbeef"},
		}
		out, err := EvaluateWorkflowCallOutputs(spec, &model.GithubContext{}, nil, nil, jobOutputs)
		require.NoError(t, err)
		assert.Equal(t, "deadbeef", out["sha"])
	})

	t.Run("output references inputs.<name>", func(t *testing.T) {
		spec := &WorkflowCallSpec{Outputs: map[string]OutputSpec{
			"target": {Value: "${{ inputs.env_name }}"},
		}}
		inputs := map[string]any{"env_name": "staging"}
		out, err := EvaluateWorkflowCallOutputs(spec, &model.GithubContext{}, nil, inputs, nil)
		require.NoError(t, err)
		assert.Equal(t, "staging", out["target"])
	})

	t.Run("multiple outputs are all evaluated", func(t *testing.T) {
		spec := &WorkflowCallSpec{Outputs: map[string]OutputSpec{
			"static":  {Value: "static-value"},
			"dynamic": {Value: "${{ vars.SUFFIX }}"},
		}}
		vars := map[string]string{"SUFFIX": "abc"}
		out, err := EvaluateWorkflowCallOutputs(spec, &model.GithubContext{}, vars, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, "static-value", out["static"])
		assert.Equal(t, "abc", out["dynamic"])
	})

	t.Run("expression referencing an undefined symbol surfaces an error", func(t *testing.T) {
		spec := &WorkflowCallSpec{Outputs: map[string]OutputSpec{
			"bad": {Value: "${{ this.is.not.valid() }}"},
		}}
		_, err := EvaluateWorkflowCallOutputs(spec, &model.GithubContext{}, nil, nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `output "bad"`)
	})
}

// callableWorkflow returns a minimal valid called-workflow YAML with on.workflow_call.
func callableWorkflow(t *testing.T, body string) []byte {
	t.Helper()
	return []byte(`name: callable
on:
  workflow_call:
    ` + body + `
jobs:
  noop:
    runs-on: ubuntu-latest
    steps:
      - run: "echo"
`)
}

// callerResults returns the minimum results map shape that NewInterpeter expects
func callerResults(callerJobID string, callerNeeds []string, deps map[string]*JobResult) map[string]*JobResult {
	out := make(map[string]*JobResult, len(deps)+1)
	maps.Copy(out, deps)
	out[callerJobID] = &JobResult{Needs: callerNeeds}
	return out
}
