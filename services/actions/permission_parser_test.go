// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/actions/jobparser"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"
)

func TestParseRawPermissions_ReadAll(t *testing.T) {
	var rawPerms yaml.Node
	err := yaml.Unmarshal([]byte(`read-all`), &rawPerms)
	assert.NoError(t, err)

	result := parseRawPermissionsExplicit(&rawPerms)
	require.NotNil(t, result)

	assert.Equal(t, perm.AccessModeRead, result.UnitAccessModes[unit.TypeCode])
	assert.Equal(t, perm.AccessModeRead, result.UnitAccessModes[unit.TypeIssues])
	assert.Equal(t, perm.AccessModeRead, result.UnitAccessModes[unit.TypePullRequests])
	assert.Equal(t, perm.AccessModeRead, result.UnitAccessModes[unit.TypePackages])
	assert.Equal(t, perm.AccessModeRead, result.UnitAccessModes[unit.TypeActions])
	assert.Equal(t, perm.AccessModeRead, result.UnitAccessModes[unit.TypeWiki])
	assert.Equal(t, perm.AccessModeRead, result.UnitAccessModes[unit.TypeProjects])
}

// TestParseRawPermissions_GithubScopes verifies that all scopes that github supports are accounted for
func TestParseRawPermissions_GithubScopes(t *testing.T) {
	var rawPerms yaml.Node
	// Taken and stripped down from:
	// https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax#defining-access-for-the-github_token-scopes
	yamlContent := `
actions: read
artifact-metadata: read
attestations: read
checks: read
contents: read
deployments: read
id-token: write
issues: read
models: read
discussions: read
packages: read
pages: read
pull-requests: read
security-events: read
statuses: read`
	err := yaml.Unmarshal([]byte(yamlContent), &rawPerms)
	require.NoError(t, err)

	result := parseRawPermissionsExplicit(&rawPerms)
	require.NotNil(t, result)

	// No asserts for permissions set on purpose
}

func TestParseRawPermissions_WriteAll(t *testing.T) {
	var rawPerms yaml.Node
	err := yaml.Unmarshal([]byte(`write-all`), &rawPerms)
	assert.NoError(t, err)

	result := parseRawPermissionsExplicit(&rawPerms)
	require.NotNil(t, result)

	assert.Equal(t, perm.AccessModeWrite, result.UnitAccessModes[unit.TypeCode])
	assert.Equal(t, perm.AccessModeWrite, result.UnitAccessModes[unit.TypeIssues])
	assert.Equal(t, perm.AccessModeWrite, result.UnitAccessModes[unit.TypePullRequests])
	assert.Equal(t, perm.AccessModeWrite, result.UnitAccessModes[unit.TypePackages])
	assert.Equal(t, perm.AccessModeWrite, result.UnitAccessModes[unit.TypeActions])
	assert.Equal(t, perm.AccessModeWrite, result.UnitAccessModes[unit.TypeWiki])
	assert.Equal(t, perm.AccessModeWrite, result.UnitAccessModes[unit.TypeProjects])
}

func TestParseRawPermissions_IndividualScopes(t *testing.T) {
	yamlContent := `
contents: write
issues: read
pull-requests: none
packages: write
actions: read
wiki: write
projects: none
`
	var rawPerms yaml.Node
	err := yaml.Unmarshal([]byte(yamlContent), &rawPerms)
	assert.NoError(t, err)

	result := parseRawPermissionsExplicit(&rawPerms)
	require.NotNil(t, result)

	assert.Equal(t, perm.AccessModeWrite, result.UnitAccessModes[unit.TypeCode])
	assert.Equal(t, perm.AccessModeRead, result.UnitAccessModes[unit.TypeIssues])
	assert.Equal(t, perm.AccessModeNone, result.UnitAccessModes[unit.TypePullRequests])
	assert.Equal(t, perm.AccessModeWrite, result.UnitAccessModes[unit.TypePackages])
	assert.Equal(t, perm.AccessModeRead, result.UnitAccessModes[unit.TypeActions])
	assert.Equal(t, perm.AccessModeWrite, result.UnitAccessModes[unit.TypeWiki])
	assert.Equal(t, perm.AccessModeNone, result.UnitAccessModes[unit.TypeProjects])
}

func TestParseRawPermissions_Priority(t *testing.T) {
	t.Run("granular-wins-over-contents", func(t *testing.T) {
		yamlContent := `
contents: read
code: write
releases: none
`
		var rawPerms yaml.Node
		err := yaml.Unmarshal([]byte(yamlContent), &rawPerms)
		assert.NoError(t, err)

		result := parseRawPermissionsExplicit(&rawPerms)
		require.NotNil(t, result)

		assert.Equal(t, perm.AccessModeWrite, result.UnitAccessModes[unit.TypeCode])
		assert.Equal(t, perm.AccessModeNone, result.UnitAccessModes[unit.TypeReleases])
	})

	t.Run("contents-applied-first", func(t *testing.T) {
		yamlContent := `
code: none
releases: write
contents: read
`
		var rawPerms yaml.Node
		err := yaml.Unmarshal([]byte(yamlContent), &rawPerms)
		assert.NoError(t, err)

		result := parseRawPermissionsExplicit(&rawPerms)
		require.NotNil(t, result)

		// code: none should win over contents: read
		assert.Equal(t, perm.AccessModeNone, result.UnitAccessModes[unit.TypeCode])
		// releases: write should win over contents: read
		assert.Equal(t, perm.AccessModeWrite, result.UnitAccessModes[unit.TypeReleases])
	})
}

func TestParseRawPermissions_EmptyNode(t *testing.T) {
	var rawPerms yaml.Node
	// Empty node

	result := parseRawPermissionsExplicit(&rawPerms)

	// Should return nil for non-explicit
	assert.Nil(t, result)
}

func TestParseRawPermissions_NilNode(t *testing.T) {
	result := parseRawPermissionsExplicit(nil)

	// Should return nil
	assert.Nil(t, result)
}

func TestParseAccessMode(t *testing.T) {
	tests := []struct {
		input    string
		expected perm.AccessMode
	}{
		{"write", perm.AccessModeWrite},
		{"read", perm.AccessModeRead},
		{"none", perm.AccessModeNone},
		{"", perm.AccessModeNone},
		{"invalid", perm.AccessModeNone},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseAccessMode(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractJobPermissionsFromWorkflow(t *testing.T) {
	workflowYAML := `
name: Test Permissions
on: workflow_dispatch
permissions: read-all

jobs:
  job-read-only:
    runs-on: ubuntu-latest
    steps:
      - run: echo "Full read-only"

  job-none-perms:
    permissions: none
    runs-on: ubuntu-latest
    steps:
      - run: echo "Full read-only"

  job-override:
    runs-on: ubuntu-latest
    permissions:
      contents: write
    steps:
      - run: echo "Override to write"
`

	expectedPerms := map[string]*repo_model.ActionsTokenPermissions{}
	expectedPerms["job-read-only"] = new(repo_model.MakeActionsTokenPermissions(perm.AccessModeRead))
	expectedPerms["job-none-perms"] = new(repo_model.MakeActionsTokenPermissions(perm.AccessModeNone))
	expectedPerms["job-override"] = new(repo_model.MakeActionsTokenPermissions(perm.AccessModeNone))
	expectedPerms["job-override"].UnitAccessModes[unit.TypeCode] = perm.AccessModeWrite
	expectedPerms["job-override"].UnitAccessModes[unit.TypeReleases] = perm.AccessModeWrite

	singleWorkflows, err := jobparser.Parse([]byte(workflowYAML))
	require.NoError(t, err)
	for _, flow := range singleWorkflows {
		jobID, jobDef := flow.Job()
		require.NotNil(t, jobDef)
		t.Run(jobID, func(t *testing.T) {
			assert.Equal(t, expectedPerms[jobID], ExtractJobPermissionsFromWorkflow(flow, jobDef))
		})
	}
}
