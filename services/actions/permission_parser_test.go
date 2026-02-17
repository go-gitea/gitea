// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	repo_model "code.gitea.io/gitea/models/repo"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestParsePermissionsFromYAMLNode_Nil(t *testing.T) {
	perms, err := ParsePermissionsFromYAMLNode(nil)
	require.NoError(t, err)
	assert.Nil(t, perms)
}

func TestParsePermissionsFromYAMLNode_Empty(t *testing.T) {
	node := &yaml.Node{}
	perms, err := ParsePermissionsFromYAMLNode(node)
	require.NoError(t, err)
	assert.Nil(t, perms)
}

func TestParsePermissionsFromYAMLNode_ReadAll(t *testing.T) {
	node := &yaml.Node{Kind: yaml.ScalarNode, Value: "read-all"}
	perms, err := ParsePermissionsFromYAMLNode(node)
	require.NoError(t, err)
	assert.NotNil(t, perms)

	for _, scope := range allKnownScopes() {
		assert.Equal(t, actions_model.TokenPermissionRead, perms[scope], "scope %s", scope)
	}
}

func TestParsePermissionsFromYAMLNode_WriteAll(t *testing.T) {
	node := &yaml.Node{Kind: yaml.ScalarNode, Value: "write-all"}
	perms, err := ParsePermissionsFromYAMLNode(node)
	require.NoError(t, err)
	assert.NotNil(t, perms)

	for _, scope := range allKnownScopes() {
		assert.Equal(t, actions_model.TokenPermissionWrite, perms[scope], "scope %s", scope)
	}
}

func TestParsePermissionsFromYAMLNode_Mapping(t *testing.T) {
	yamlStr := `
contents: read
issues: write
pull-requests: none
`
	var node yaml.Node
	err := yaml.Unmarshal([]byte(yamlStr), &node)
	require.NoError(t, err)
	// yaml.Unmarshal wraps in a document node
	perms, err := ParsePermissionsFromYAMLNode(node.Content[0])
	require.NoError(t, err)
	assert.NotNil(t, perms)

	assert.Equal(t, actions_model.TokenPermissionRead, perms["contents"])
	assert.Equal(t, actions_model.TokenPermissionWrite, perms["issues"])
	assert.Equal(t, actions_model.TokenPermissionNone, perms["pull-requests"])
}

func TestParsePermissionsFromYAMLNode_UnknownScope(t *testing.T) {
	yamlStr := `
contents: read
unknown-scope: write
`
	var node yaml.Node
	err := yaml.Unmarshal([]byte(yamlStr), &node)
	require.NoError(t, err)
	perms, err := ParsePermissionsFromYAMLNode(node.Content[0])
	require.NoError(t, err)
	assert.NotNil(t, perms)

	// Unknown scope is ignored
	assert.Equal(t, actions_model.TokenPermissionRead, perms["contents"])
	_, exists := perms["unknown-scope"]
	assert.False(t, exists)
}

func TestParsePermissionsFromYAMLNode_InvalidLevel(t *testing.T) {
	yamlStr := `
contents: admin
`
	var node yaml.Node
	err := yaml.Unmarshal([]byte(yamlStr), &node)
	require.NoError(t, err)
	_, err = ParsePermissionsFromYAMLNode(node.Content[0])
	assert.Error(t, err)
}

func TestComputeJobPermissions_DefaultPermissive(t *testing.T) {
	cfg := &repo_model.ActionsConfig{
		DefaultTokenPermission: repo_model.ActionsTokenPermissionPermissive,
	}
	perms, err := ComputeJobPermissions(cfg, nil, nil, false)
	require.NoError(t, err)
	assert.NotNil(t, perms)

	for _, scope := range allKnownScopes() {
		assert.Equal(t, actions_model.TokenPermissionWrite, perms[scope], "scope %s", scope)
	}
}

func TestComputeJobPermissions_DefaultRestricted(t *testing.T) {
	cfg := &repo_model.ActionsConfig{
		DefaultTokenPermission: repo_model.ActionsTokenPermissionRestricted,
	}
	perms, err := ComputeJobPermissions(cfg, nil, nil, false)
	require.NoError(t, err)
	assert.NotNil(t, perms)

	assert.Equal(t, actions_model.TokenPermissionRead, perms["contents"])
	assert.Equal(t, actions_model.TokenPermissionRead, perms["packages"])
	assert.Equal(t, actions_model.TokenPermissionNone, perms["issues"])
	assert.Equal(t, actions_model.TokenPermissionNone, perms["pull-requests"])
	assert.Equal(t, actions_model.TokenPermissionNone, perms["actions"])
}

func TestComputeJobPermissions_ForkPullRequest(t *testing.T) {
	cfg := &repo_model.ActionsConfig{
		DefaultTokenPermission: repo_model.ActionsTokenPermissionPermissive,
	}
	perms, err := ComputeJobPermissions(cfg, nil, nil, true)
	require.NoError(t, err)
	assert.NotNil(t, perms)

	// Fork PRs should be capped at read
	for _, scope := range allKnownScopes() {
		assert.LessOrEqual(t, string(perms[scope]), string(actions_model.TokenPermissionRead), "scope %s", scope)
	}
}

func TestComputeJobPermissions_JobOverridesWorkflow(t *testing.T) {
	cfg := &repo_model.ActionsConfig{
		DefaultTokenPermission: repo_model.ActionsTokenPermissionPermissive,
	}

	// Workflow says read-all
	wfNode := &yaml.Node{Kind: yaml.ScalarNode, Value: "read-all"}

	// Job says contents: write
	jobYaml := `
contents: write
`
	var jobDoc yaml.Node
	err := yaml.Unmarshal([]byte(jobYaml), &jobDoc)
	require.NoError(t, err)
	jobNode := jobDoc.Content[0]

	perms, err := ComputeJobPermissions(cfg, wfNode, jobNode, false)
	require.NoError(t, err)
	assert.NotNil(t, perms)

	// Job overrides workflow, so contents should be write
	assert.Equal(t, actions_model.TokenPermissionWrite, perms["contents"])
	// All other scopes not mentioned in job -> none (GitHub behavior)
	assert.Equal(t, actions_model.TokenPermissionNone, perms["issues"])
	assert.Equal(t, actions_model.TokenPermissionNone, perms["pull-requests"])
}

func TestComputeJobPermissions_WorkflowOverridesDefault(t *testing.T) {
	cfg := &repo_model.ActionsConfig{
		DefaultTokenPermission: repo_model.ActionsTokenPermissionPermissive,
	}

	wfNode := &yaml.Node{Kind: yaml.ScalarNode, Value: "read-all"}

	perms, err := ComputeJobPermissions(cfg, wfNode, nil, false)
	require.NoError(t, err)
	assert.NotNil(t, perms)

	// Workflow overrides default, everything should be read
	for _, scope := range allKnownScopes() {
		assert.Equal(t, actions_model.TokenPermissionRead, perms[scope], "scope %s", scope)
	}
}

func TestComputeJobPermissions_ForkPRClampsJobPerms(t *testing.T) {
	cfg := &repo_model.ActionsConfig{
		DefaultTokenPermission: repo_model.ActionsTokenPermissionPermissive,
	}

	// Job says write for contents
	jobYaml := `
contents: write
issues: write
`
	var jobDoc yaml.Node
	err := yaml.Unmarshal([]byte(jobYaml), &jobDoc)
	require.NoError(t, err)
	jobNode := jobDoc.Content[0]

	perms, err := ComputeJobPermissions(cfg, nil, jobNode, true)
	require.NoError(t, err)
	assert.NotNil(t, perms)

	// Fork PR clamps everything to read
	assert.Equal(t, actions_model.TokenPermissionRead, perms["contents"])
	assert.Equal(t, actions_model.TokenPermissionRead, perms["issues"])
}

func TestMarshalUnmarshalTokenPermissions(t *testing.T) {
	original := actions_model.TokenPermissions{
		"contents":      actions_model.TokenPermissionWrite,
		"issues":        actions_model.TokenPermissionRead,
		"pull-requests": actions_model.TokenPermissionNone,
	}

	data, err := actions_model.MarshalTokenPermissions(original)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	restored, err := actions_model.UnmarshalTokenPermissions(data)
	require.NoError(t, err)
	assert.Equal(t, original, restored)
}

func TestMarshalUnmarshalTokenPermissions_Empty(t *testing.T) {
	data, err := actions_model.MarshalTokenPermissions(nil)
	require.NoError(t, err)
	assert.Empty(t, data)

	restored, err := actions_model.UnmarshalTokenPermissions("")
	require.NoError(t, err)
	assert.Nil(t, restored)
}

func TestApplyPermissions_UnmentionedScopesGetNone(t *testing.T) {
	base := actions_model.TokenPermissions{
		"contents":      actions_model.TokenPermissionWrite,
		"issues":        actions_model.TokenPermissionWrite,
		"pull-requests": actions_model.TokenPermissionWrite,
		"packages":      actions_model.TokenPermissionWrite,
		"actions":       actions_model.TokenPermissionWrite,
	}
	overrides := actions_model.TokenPermissions{
		"contents": actions_model.TokenPermissionRead,
	}

	result := applyPermissions(base, overrides)

	assert.Equal(t, actions_model.TokenPermissionRead, result["contents"])
	// Unmentioned scopes get none
	assert.Equal(t, actions_model.TokenPermissionNone, result["issues"])
	assert.Equal(t, actions_model.TokenPermissionNone, result["pull-requests"])
	assert.Equal(t, actions_model.TokenPermissionNone, result["packages"])
	assert.Equal(t, actions_model.TokenPermissionNone, result["actions"])
}

func TestClampPermissions(t *testing.T) {
	perms := actions_model.TokenPermissions{
		"contents": actions_model.TokenPermissionWrite,
		"issues":   actions_model.TokenPermissionWrite,
	}
	maxPerms := actions_model.TokenPermissions{
		"contents": actions_model.TokenPermissionRead,
		"issues":   actions_model.TokenPermissionWrite,
	}

	result := clampPermissions(perms, maxPerms)

	assert.Equal(t, actions_model.TokenPermissionRead, result["contents"])
	assert.Equal(t, actions_model.TokenPermissionWrite, result["issues"])
}
