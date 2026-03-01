// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"

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

	assert.Equal(t, perm.AccessModeRead, result.Code)
	assert.Equal(t, perm.AccessModeRead, result.Issues)
	assert.Equal(t, perm.AccessModeRead, result.PullRequests)
	assert.Equal(t, perm.AccessModeRead, result.Packages)
	assert.Equal(t, perm.AccessModeRead, result.Actions)
	assert.Equal(t, perm.AccessModeRead, result.Wiki)
	assert.Equal(t, perm.AccessModeRead, result.Projects)
}

func TestParseRawPermissions_WriteAll(t *testing.T) {
	var rawPerms yaml.Node
	err := yaml.Unmarshal([]byte(`write-all`), &rawPerms)
	assert.NoError(t, err)

	result := parseRawPermissionsExplicit(&rawPerms)
	require.NotNil(t, result)

	assert.Equal(t, perm.AccessModeWrite, result.Code)
	assert.Equal(t, perm.AccessModeWrite, result.Issues)
	assert.Equal(t, perm.AccessModeWrite, result.PullRequests)
	assert.Equal(t, perm.AccessModeWrite, result.Packages)
	assert.Equal(t, perm.AccessModeWrite, result.Actions)
	assert.Equal(t, perm.AccessModeWrite, result.Wiki)
	assert.Equal(t, perm.AccessModeWrite, result.Projects)
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

	assert.Equal(t, perm.AccessModeWrite, result.Code)
	assert.Equal(t, perm.AccessModeRead, result.Issues)
	assert.Equal(t, perm.AccessModeNone, result.PullRequests)
	assert.Equal(t, perm.AccessModeWrite, result.Packages)
	assert.Equal(t, perm.AccessModeRead, result.Actions)
	assert.Equal(t, perm.AccessModeWrite, result.Wiki)
	assert.Equal(t, perm.AccessModeNone, result.Projects)
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

		assert.Equal(t, perm.AccessModeWrite, result.Code)
		assert.Equal(t, perm.AccessModeNone, result.Releases)
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
		assert.Equal(t, perm.AccessModeNone, result.Code)
		// releases: write should win over contents: read
		assert.Equal(t, perm.AccessModeWrite, result.Releases)
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

func TestMarshalUnmarshalTokenPermissions(t *testing.T) {
	original := repo_model.ActionsTokenPermissions{
		Code:         perm.AccessModeWrite,
		Issues:       perm.AccessModeRead,
		PullRequests: perm.AccessModeNone,
		Packages:     perm.AccessModeWrite,
		Actions:      perm.AccessModeRead,
		Wiki:         perm.AccessModeWrite,
		Projects:     perm.AccessModeRead,
	}

	// Marshal
	jsonStr, err := repo_model.MarshalTokenPermissions(original)
	assert.NoError(t, err)
	assert.NotEmpty(t, jsonStr)

	// Unmarshal
	result, err := repo_model.UnmarshalTokenPermissions(jsonStr)
	assert.NoError(t, err)
	assert.Equal(t, original, result)
}

func TestUnmarshalTokenPermissions_EmptyString(t *testing.T) {
	result, err := repo_model.UnmarshalTokenPermissions("")
	assert.NoError(t, err)
	// Should return zero-value struct
	assert.Equal(t, perm.AccessModeNone, result.Code)
	assert.Equal(t, perm.AccessModeNone, result.Issues)
}
