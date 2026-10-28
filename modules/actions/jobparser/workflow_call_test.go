// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jobparser

import (
	"testing"

	"gitea.dev/actionslib/pkg/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"
)

func TestResolveCallerInputs(t *testing.T) {
	config := &model.WorkflowCall{Inputs: map[string]model.WorkflowCallInput{
		"env":         {Type: "string"},
		"from_inputs": {Type: "string"},
		"from_needs":  {Type: "string"},
		"count":       {Type: "number"},
		"index":       {Type: "number"},
	}}
	results := map[string]*JobResult{
		"caller":   {Needs: []string{"upstream"}},
		"upstream": {Result: "success", Outputs: map[string]string{"commit": "abc123"}},
	}
	var job Job
	require.NoError(t, yaml.Unmarshal([]byte(`strategy: {matrix: {target: [staging]}, job-index: 1, job-total: 2}
with: {env: "${{ matrix.target }}", from_inputs: "${{ inputs.PARENT_VAR }}", from_needs: "${{ needs.upstream.outputs.commit }}", count: 42, index: "${{ strategy.job-index }}"}`), &job))
	out, err := ResolveCallerInputs("caller", &job, config, map[string]any{"event": map[string]any{}}, results,
		nil, map[string]any{"PARENT_VAR": "from-parent"})
	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"env": "staging", "from_inputs": "from-parent", "from_needs": "abc123", "count": 42.0, "index": 1.0,
	}, out)

	for _, tt := range []struct {
		with    string
		inputs  map[string]model.WorkflowCallInput
		want    map[string]any
		wantErr bool
	}{
		{with: "with: {EXTRA: value}", want: map[string]any{}},
		{with: `with: ${{ fromJSON('{"env":"prod"}') }}`, wantErr: true},
		{inputs: map[string]model.WorkflowCallInput{"required": {Type: "string", Required: true, Default: "fallback"}}, want: map[string]any{"required": "fallback"}},
		{inputs: map[string]model.WorkflowCallInput{"derived": {Type: "string", Default: "${{ inputs.PARENT_VAR }}"}}, want: map[string]any{"derived": "from-parent"}},
		{with: "with: {value: 'false'}", inputs: map[string]model.WorkflowCallInput{"value": {Type: "boolean"}}, wantErr: true},
		{with: `with: {value: "${{ '5' }}"}`, inputs: map[string]model.WorkflowCallInput{"value": {Type: "number"}}, wantErr: true},
	} {
		job = Job{}
		require.NoError(t, yaml.Unmarshal([]byte(tt.with), &job))
		out, err := ResolveCallerInputs("caller", &job, &model.WorkflowCall{Inputs: tt.inputs}, nil, nil, nil, map[string]any{"PARENT_VAR": "from-parent"})
		if tt.wantErr {
			require.Error(t, err, tt.with)
			continue
		}
		require.NoError(t, err)
		assert.Equal(t, tt.want, out)
	}
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
