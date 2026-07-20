// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScopedWorkflowSource_IsWorkflowRequired(t *testing.T) {
	src := &ActionScopedWorkflowSource{WorkflowConfigs: map[string]*ScopedWorkflowConfig{
		"a.yml": {Required: true, Patterns: []string{"p"}},
		"b.yml": {Required: true, Patterns: []string{"p"}},
		"c.yml": {Required: false, Patterns: []string{"p"}}, // patterns kept as history, not required
	}}
	assert.True(t, src.IsWorkflowRequired("a.yml"))
	assert.True(t, src.IsWorkflowRequired("b.yml"))
	assert.False(t, src.IsWorkflowRequired("c.yml"), "config kept as history but not required")
	assert.False(t, src.IsWorkflowRequired("d.yml"))

	empty := &ActionScopedWorkflowSource{}
	assert.False(t, empty.IsWorkflowRequired("a.yml"))
}

func TestIsWorkflowRequiredInSources(t *testing.T) {
	// repo 100 registered twice (org optional + instance required).
	sources := []*ActionScopedWorkflowSource{
		{OwnerID: 2, SourceRepoID: 100, WorkflowConfigs: nil},
		{OwnerID: 0, SourceRepoID: 100, WorkflowConfigs: map[string]*ScopedWorkflowConfig{"a.yml": {Required: true, Patterns: []string{"p"}}}},
		{OwnerID: 0, SourceRepoID: 200, WorkflowConfigs: map[string]*ScopedWorkflowConfig{"b.yml": {Required: true, Patterns: []string{"p"}}}},
	}

	assert.True(t, IsWorkflowRequiredInSources(sources, 100, "a.yml"), "required at instance level wins over org optional")
	assert.False(t, IsWorkflowRequiredInSources(sources, 100, "z.yml"))
	assert.False(t, IsWorkflowRequiredInSources(sources, 200, "a.yml"), "a.yml is required for repo 100, not repo 200")
	assert.True(t, IsWorkflowRequiredInSources(sources, 200, "b.yml"))
	assert.False(t, IsWorkflowRequiredInSources(sources, 999, "a.yml"), "unknown source repo")
}

func TestGetEffectiveScopedWorkflowSources(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	rows := []*ActionScopedWorkflowSource{
		{OwnerID: 2, SourceRepoID: 100, WorkflowConfigs: nil}, // org 2 registers repo 100 (optional)
		{OwnerID: 0, SourceRepoID: 100, WorkflowConfigs: map[string]*ScopedWorkflowConfig{"a.yml": {Required: true, Patterns: []string{"p"}}}}, // instance also registers repo 100 (required)
		{OwnerID: 0, SourceRepoID: 200, WorkflowConfigs: map[string]*ScopedWorkflowConfig{"b.yml": {Required: true, Patterns: []string{"p"}}}}, // instance source 200
		{OwnerID: 3, SourceRepoID: 300, WorkflowConfigs: map[string]*ScopedWorkflowConfig{"c.yml": {Required: true, Patterns: []string{"p"}}}}, // a different owner's source
	}
	for _, r := range rows {
		require.NoError(t, db.Insert(ctx, r))
	}

	// owner 2 sees its own sources plus instance-level ones, but not owner 3's.
	owner2, err := GetEffectiveScopedWorkflowSources(ctx, 2)
	require.NoError(t, err)
	assert.Len(t, owner2, 3)

	required, err := IsScopedWorkflowRequired(ctx, 2, 100, "a.yml")
	require.NoError(t, err)
	assert.True(t, required, "instance marks a.yml required → required for owner 2 even though org left it optional")

	required, err = IsScopedWorkflowRequired(ctx, 2, 100, "x.yml")
	require.NoError(t, err)
	assert.False(t, required)

	required, err = IsScopedWorkflowRequired(ctx, 2, 200, "b.yml")
	require.NoError(t, err)
	assert.True(t, required)

	// owner 3's source must not be effective for owner 2.
	required, err = IsScopedWorkflowRequired(ctx, 2, 300, "c.yml")
	require.NoError(t, err)
	assert.False(t, required)

	// IsScopedWorkflowSourceEffective: owner-level and instance-level sources are effective; another owner's is not.
	effective, err := IsScopedWorkflowSourceEffective(ctx, 2, 100)
	require.NoError(t, err)
	assert.True(t, effective, "owner 2's own source")

	effective, err = IsScopedWorkflowSourceEffective(ctx, 2, 200)
	require.NoError(t, err)
	assert.True(t, effective, "instance-level source is effective for any owner")

	effective, err = IsScopedWorkflowSourceEffective(ctx, 2, 300)
	require.NoError(t, err)
	assert.False(t, effective, "owner 3's source is not effective for owner 2")

	effective, err = IsScopedWorkflowSourceEffective(ctx, 2, 999)
	require.NoError(t, err)
	assert.False(t, effective, "unknown source repo")

	effective, err = IsScopedWorkflowSourceEffective(ctx, 3, 300)
	require.NoError(t, err)
	assert.True(t, effective, "owner 3's own source is effective for owner 3")
}

func TestScopedWorkflowSourceCRUD(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	// add is idempotent
	require.NoError(t, AddScopedWorkflowSource(ctx, 5, 10))
	require.NoError(t, AddScopedWorkflowSource(ctx, 5, 10))
	sources, err := GetScopedWorkflowSourcesByOwner(ctx, 5)
	require.NoError(t, err)
	assert.Len(t, sources, 1)

	// set the per-workflow configs (entry name -> {required, patterns}); a.yml required, b.yml kept as history (not required)
	configs := map[string]*ScopedWorkflowConfig{
		"a.yml": {Required: true, Patterns: []string{"src: a.yml / *"}},
		"b.yml": {Required: false, Patterns: []string{"src: b.yml / build (push)"}},
	}
	require.NoError(t, SetScopedWorkflowSourceConfigs(ctx, 5, 10, configs))
	src, err := GetScopedWorkflowSource(ctx, 5, 10)
	require.NoError(t, err)
	assert.Equal(t, configs, src.WorkflowConfigs)

	// clearing the configs works
	require.NoError(t, SetScopedWorkflowSourceConfigs(ctx, 5, 10, nil))
	src, err = GetScopedWorkflowSource(ctx, 5, 10)
	require.NoError(t, err)
	assert.Empty(t, src.WorkflowConfigs)

	// remove
	require.NoError(t, RemoveScopedWorkflowSource(ctx, 5, 10))
	_, err = GetScopedWorkflowSource(ctx, 5, 10)
	assert.ErrorIs(t, err, util.ErrNotExist)
	sources, err = GetScopedWorkflowSourcesByOwner(ctx, 5)
	require.NoError(t, err)
	assert.Empty(t, sources)
}
