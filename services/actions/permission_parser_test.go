// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestParseRawPermissions_ReadAll(t *testing.T) {
	var rawPerms yaml.Node
	err := yaml.Unmarshal([]byte(`read-all`), &rawPerms)
	assert.NoError(t, err)

	defaultPerms := repo_model.DefaultActionsTokenPermissions(repo_model.ActionsTokenPermissionModePermissive)
	result := parseRawPermissions(&rawPerms, defaultPerms)

	assert.Equal(t, perm.AccessModeRead, result.Contents)
	assert.Equal(t, perm.AccessModeRead, result.Issues)
	assert.Equal(t, perm.AccessModeRead, result.PullRequests)
	assert.Equal(t, perm.AccessModeRead, result.Packages)
	assert.Equal(t, perm.AccessModeRead, result.Actions)
	assert.Equal(t, perm.AccessModeRead, result.Wiki)
}

func TestParseRawPermissions_WriteAll(t *testing.T) {
	var rawPerms yaml.Node
	err := yaml.Unmarshal([]byte(`write-all`), &rawPerms)
	assert.NoError(t, err)

	defaultPerms := repo_model.DefaultActionsTokenPermissions(repo_model.ActionsTokenPermissionModeRestricted)
	result := parseRawPermissions(&rawPerms, defaultPerms)

	assert.Equal(t, perm.AccessModeWrite, result.Contents)
	assert.Equal(t, perm.AccessModeWrite, result.Issues)
	assert.Equal(t, perm.AccessModeWrite, result.PullRequests)
	assert.Equal(t, perm.AccessModeWrite, result.Packages)
	assert.Equal(t, perm.AccessModeWrite, result.Actions)
	assert.Equal(t, perm.AccessModeWrite, result.Wiki)
}

func TestParseRawPermissions_IndividualScopes(t *testing.T) {
	yamlContent := `
contents: write
issues: read
pull-requests: none
packages: write
actions: read
wiki: write
`
	var rawPerms yaml.Node
	err := yaml.Unmarshal([]byte(yamlContent), &rawPerms)
	assert.NoError(t, err)

	defaultPerms := repo_model.ActionsTokenPermissions{
		Contents:     perm.AccessModeNone,
		Issues:       perm.AccessModeNone,
		PullRequests: perm.AccessModeNone,
		Packages:     perm.AccessModeNone,
		Actions:      perm.AccessModeNone,
		Wiki:         perm.AccessModeNone,
	}
	result := parseRawPermissions(&rawPerms, defaultPerms)

	assert.Equal(t, perm.AccessModeWrite, result.Contents)
	assert.Equal(t, perm.AccessModeRead, result.Issues)
	assert.Equal(t, perm.AccessModeNone, result.PullRequests)
	assert.Equal(t, perm.AccessModeWrite, result.Packages)
	assert.Equal(t, perm.AccessModeRead, result.Actions)
	assert.Equal(t, perm.AccessModeWrite, result.Wiki)
}

func TestParseRawPermissions_PartialOverride(t *testing.T) {
	yamlContent := `
contents: read
issues: write
`
	var rawPerms yaml.Node
	err := yaml.Unmarshal([]byte(yamlContent), &rawPerms)
	assert.NoError(t, err)

	// Defaults are write for everything
	defaultPerms := repo_model.DefaultActionsTokenPermissions(repo_model.ActionsTokenPermissionModePermissive)
	result := parseRawPermissions(&rawPerms, defaultPerms)

	// Overridden scopes
	assert.Equal(t, perm.AccessModeRead, result.Contents)
	assert.Equal(t, perm.AccessModeWrite, result.Issues)
	// Non-overridden scopes keep defaults
	assert.Equal(t, perm.AccessModeWrite, result.PullRequests)
	assert.Equal(t, perm.AccessModeRead, result.Packages) // Packages default to read in permissive
	assert.Equal(t, perm.AccessModeWrite, result.Actions)
	assert.Equal(t, perm.AccessModeWrite, result.Wiki)
}

func TestParseRawPermissions_EmptyNode(t *testing.T) {
	var rawPerms yaml.Node
	// Empty node

	defaultPerms := repo_model.DefaultActionsTokenPermissions(repo_model.ActionsTokenPermissionModePermissive)
	result := parseRawPermissions(&rawPerms, defaultPerms)

	// Should return defaults
	assert.Equal(t, defaultPerms.Contents, result.Contents)
	assert.Equal(t, defaultPerms.Issues, result.Issues)
}

func TestParseRawPermissions_NilNode(t *testing.T) {
	defaultPerms := repo_model.DefaultActionsTokenPermissions(repo_model.ActionsTokenPermissionModePermissive)
	result := parseRawPermissions(nil, defaultPerms)

	// Should return defaults
	assert.Equal(t, defaultPerms.Contents, result.Contents)
	assert.Equal(t, defaultPerms.Issues, result.Issues)
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
		Contents:     perm.AccessModeWrite,
		Issues:       perm.AccessModeRead,
		PullRequests: perm.AccessModeNone,
		Packages:     perm.AccessModeWrite,
		Actions:      perm.AccessModeRead,
		Wiki:         perm.AccessModeWrite,
	}

	// Marshal
	jsonStr := repo_model.MarshalTokenPermissions(original)
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
	assert.Equal(t, perm.AccessModeNone, result.Contents)
	assert.Equal(t, perm.AccessModeNone, result.Issues)
}
