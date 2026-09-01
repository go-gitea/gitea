// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionsConfig_ScopedWorkflowOptOut(t *testing.T) {
	cfg := &ActionsConfig{}

	assert.False(t, cfg.IsScopedWorkflowDisabled(100, "ci.yml"))

	cfg.DisableScopedWorkflow(100, "ci.yml")
	assert.True(t, cfg.IsScopedWorkflowDisabled(100, "ci.yml"))

	// idempotent
	cfg.DisableScopedWorkflow(100, "ci.yml")
	assert.Len(t, cfg.DisabledScopedWorkflows, 1)

	// keyed by source repo: the same filename from a different source repo is independent
	assert.False(t, cfg.IsScopedWorkflowDisabled(200, "ci.yml"))

	// must not collide with the repo-level DisabledWorkflows list (bare filename)
	assert.False(t, cfg.IsWorkflowDisabled("ci.yml"))
	cfg.DisableWorkflow("ci.yml")
	assert.True(t, cfg.IsWorkflowDisabled("ci.yml"))
	assert.True(t, cfg.IsScopedWorkflowDisabled(100, "ci.yml"), "repo-level disable must not touch the scoped entry")

	cfg.EnableScopedWorkflow(100, "ci.yml")
	assert.False(t, cfg.IsScopedWorkflowDisabled(100, "ci.yml"))
	assert.True(t, cfg.IsWorkflowDisabled("ci.yml"), "enabling the scoped entry must not touch the repo-level disable")
}

func TestActionsConfig_ScopedWorkflowSerialization(t *testing.T) {
	cfg := &ActionsConfig{}
	cfg.DisableScopedWorkflow(100, "ci.yml")
	cfg.DisableWorkflow("repo.yml")

	bs, err := cfg.ToDB()
	require.NoError(t, err)

	got := &ActionsConfig{}
	require.NoError(t, got.FromDB(bs))
	assert.True(t, got.IsScopedWorkflowDisabled(100, "ci.yml"))
	assert.True(t, got.IsWorkflowDisabled("repo.yml"))
}
