// Copyright 2024 The Gitea Authors.
// All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"testing"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/commitstatus"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeRequiredContextsCommitStatus(t *testing.T) {
	cases := []struct {
		commitStatuses   []*git_model.CommitStatus
		requiredContexts []string
		expected         commitstatus.CommitStatusState
	}{
		{
			commitStatuses:   []*git_model.CommitStatus{},
			requiredContexts: []string{},
			expected:         commitstatus.CommitStatusPending,
		},
		{
			commitStatuses: []*git_model.CommitStatus{
				{Context: "Build xxx", State: commitstatus.CommitStatusSkipped},
			},
			requiredContexts: []string{"Build*"},
			expected:         commitstatus.CommitStatusSuccess,
		},
		{
			commitStatuses: []*git_model.CommitStatus{
				{Context: "Build 1", State: commitstatus.CommitStatusSkipped},
				{Context: "Build 2", State: commitstatus.CommitStatusSuccess},
				{Context: "Build 3", State: commitstatus.CommitStatusSuccess},
			},
			requiredContexts: []string{"Build*"},
			expected:         commitstatus.CommitStatusSuccess,
		},
		{
			commitStatuses: []*git_model.CommitStatus{
				{Context: "Build 1", State: commitstatus.CommitStatusSuccess},
				{Context: "Build 2", State: commitstatus.CommitStatusSuccess},
				{Context: "Build 2t", State: commitstatus.CommitStatusPending},
			},
			requiredContexts: []string{"Build*", "Build 2t*"},
			expected:         commitstatus.CommitStatusPending,
		},
		{
			commitStatuses: []*git_model.CommitStatus{
				{Context: "Build 1", State: commitstatus.CommitStatusSuccess},
				{Context: "Build 2", State: commitstatus.CommitStatusSuccess},
				{Context: "Build 2t", State: commitstatus.CommitStatusFailure},
			},
			requiredContexts: []string{"Build*", "Build 2t*"},
			expected:         commitstatus.CommitStatusFailure,
		},
		{
			commitStatuses: []*git_model.CommitStatus{
				{Context: "Build 1", State: commitstatus.CommitStatusSuccess},
				{Context: "Build 2", State: commitstatus.CommitStatusSuccess},
				{Context: "Build 2t", State: commitstatus.CommitStatusFailure},
			},
			requiredContexts: []string{"Build*"},
			expected:         commitstatus.CommitStatusFailure,
		},
		{
			commitStatuses: []*git_model.CommitStatus{
				{Context: "Build 1", State: commitstatus.CommitStatusSuccess},
				{Context: "Build 2", State: commitstatus.CommitStatusSuccess},
				{Context: "Build 2t", State: commitstatus.CommitStatusSuccess},
			},
			requiredContexts: []string{"Build*", "Build 2t*", "Build 3*"},
			expected:         commitstatus.CommitStatusPending,
		},
		{
			commitStatuses: []*git_model.CommitStatus{
				{Context: "Build 1", State: commitstatus.CommitStatusSuccess},
				{Context: "Build 2", State: commitstatus.CommitStatusSuccess},
				{Context: "Build 2t", State: commitstatus.CommitStatusSuccess},
			},
			requiredContexts: []string{"Build*", "Build *", "Build 2t*", "Build 1*"},
			expected:         commitstatus.CommitStatusSuccess,
		},
	}
	for i, c := range cases {
		assert.Equal(t, c.expected, MergeRequiredContextsCommitStatus(c.commitStatuses, c.requiredContexts), "case %d", i)
	}
}

// TestEffectiveRequiredContexts: every required scoped workflow's stored status-check patterns are appended to the
// branch protection's configured contexts unconditionally (must-present; the matching is done downstream).
func TestEffectiveRequiredContexts(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	consumer := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4}) // owned by user5
	pbOn := &git_model.ProtectedBranch{EnableStatusCheck: true, StatusCheckContexts: []string{"configured/check"}}

	t.Run("nil protected branch: nil", func(t *testing.T) {
		got, err := EffectiveRequiredContexts(t.Context(), consumer, nil)
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("status checks disabled, no required scoped: nothing required", func(t *testing.T) {
		pbOff := &git_model.ProtectedBranch{EnableStatusCheck: false, StatusCheckContexts: []string{"configured/check"}}
		got, err := EffectiveRequiredContexts(t.Context(), consumer, pbOff)
		require.NoError(t, err)
		assert.Empty(t, got) // the rule's own status check is off and no required scoped workflow applies -> nothing gates
	})

	t.Run("owner with no scoped sources: configured contexts unchanged", func(t *testing.T) {
		noSourceRepo := &repo_model.Repository{ID: consumer.ID, OwnerID: 99999}
		got, err := EffectiveRequiredContexts(t.Context(), noSourceRepo, pbOn)
		require.NoError(t, err)
		assert.Equal(t, []string{"configured/check"}, got)
	})

	t.Run("required workflow patterns appended", func(t *testing.T) {
		require.NoError(t, db.Insert(t.Context(), &actions_model.ActionScopedWorkflowSource{
			OwnerID:      consumer.OwnerID,
			SourceRepoID: 1,
			WorkflowConfigs: map[string]*actions_model.ScopedWorkflowConfig{
				"ci.yaml":  {Required: true, Patterns: []string{"org/src: ci.yaml / build (pull_request)", "org/src: ci.yaml / lint (pull_request)"}},
				"old.yaml": {Required: false, Patterns: []string{"org/src: old.yaml / *"}}, // kept as history, must NOT be enforced
			},
		}))
		// No status is passed/needed: required patterns are enforced even though nothing has posted them yet (must-present).
		got, err := EffectiveRequiredContexts(t.Context(), consumer, pbOn)
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{
			"configured/check",
			"org/src: ci.yaml / build (pull_request)",
			"org/src: ci.yaml / lint (pull_request)",
		}, got)
		assert.NotContains(t, got, "org/src: old.yaml / *", "a non-required (history) config must not be enforced")
	})

	t.Run("status checks disabled, with required scoped: only the scoped patterns gate", func(t *testing.T) {
		pbOff := &git_model.ProtectedBranch{EnableStatusCheck: false, StatusCheckContexts: []string{"configured/check"}}
		got, err := EffectiveRequiredContexts(t.Context(), consumer, pbOff)
		require.NoError(t, err)
		// "configured/check" is dropped (the rule's own status check is off); only the required scoped patterns remain.
		assert.ElementsMatch(t, []string{
			"org/src: ci.yaml / build (pull_request)",
			"org/src: ci.yaml / lint (pull_request)",
		}, got)
	})
}
