// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package secret

import (
	"testing"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetScopedSecretsForJob(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	base := map[string]string{
		"GITHUB_TOKEN": "tok",
		"GITEA_TOKEN":  "tok",
		"PROD_API_KEY": "prod-secret",
		"DEV_API_KEY":  "dev-secret",
	}

	// insertCaller create an ActionRunJob caller row with the given CallSecrets policy
	insertCaller := func(t *testing.T, runID, parentJobID int64, callSecrets string) *actions_model.ActionRunJob {
		t.Helper()
		job := &actions_model.ActionRunJob{
			RunID:            runID,
			RepoID:           1,
			IsReusableCaller: true,
			ParentJobID:      parentJobID,
			CallSecrets:      callSecrets,
			Status:           actions_model.StatusBlocked,
		}
		require.NoError(t, db.Insert(t.Context(), job))
		return job
	}

	t.Run("TopLevelJob_ReturnsBaseUnchanged", func(t *testing.T) {
		const runID = 9001
		leaf := &actions_model.ActionRunJob{RunID: runID, ParentJobID: 0}

		got, err := getScopedSecretsForJob(ctx, leaf, base)
		require.NoError(t, err)
		assert.Equal(t, base, got, "top-level jobs should see the full base scope")
	})

	t.Run("CallerInherit_PassesParentScopeThrough", func(t *testing.T) {
		const runID = 9002
		caller := insertCaller(t, runID, 0, "inherit")
		leaf := &actions_model.ActionRunJob{RunID: runID, ParentJobID: caller.ID}

		got, err := getScopedSecretsForJob(ctx, leaf, base)
		require.NoError(t, err)
		assert.Equal(t, base, got, "secrets: inherit forwards everything from parent scope")
	})

	t.Run("CallerEmptySecrets_ExposesOnlyAutoTokens", func(t *testing.T) {
		const runID = 9003
		caller := insertCaller(t, runID, 0, "")
		leaf := &actions_model.ActionRunJob{RunID: runID, ParentJobID: caller.ID}

		got, err := getScopedSecretsForJob(ctx, leaf, base)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"GITHUB_TOKEN": "tok",
			"GITEA_TOKEN":  "tok",
		}, got)
	})

	t.Run("CallerMapping_OnlyMappedAliasesPlusTokens", func(t *testing.T) {
		const runID = 9004
		// {alias: source} - the called workflow sees `secrets.MY_KEY` resolved to PROD_API_KEY's value.
		caller := insertCaller(t, runID, 0, `{"MY_KEY":"PROD_API_KEY"}`)
		leaf := &actions_model.ActionRunJob{RunID: runID, ParentJobID: caller.ID}

		got, err := getScopedSecretsForJob(ctx, leaf, base)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"GITHUB_TOKEN": "tok",
			"GITEA_TOKEN":  "tok",
			"MY_KEY":       "prod-secret",
			// no "dev-secret"
		}, got)
	})

	t.Run("CallerMapping_CaseInsensitiveSource", func(t *testing.T) {
		const runID = 9005
		caller := insertCaller(t, runID, 0, `{"alias":"prod_api_key"}`)
		leaf := &actions_model.ActionRunJob{RunID: runID, ParentJobID: caller.ID}

		got, err := getScopedSecretsForJob(ctx, leaf, base)
		require.NoError(t, err)
		assert.Equal(t, "prod-secret", got["alias"])
	})

	t.Run("CallerMapping_UnknownSourceDropsAlias", func(t *testing.T) {
		const runID = 9006
		// alias points at a non-existent secret name, so it must be dropped.
		caller := insertCaller(t, runID, 0, `{"MAPPED_ALIAS":"DOES_NOT_EXIST"}`)
		leaf := &actions_model.ActionRunJob{RunID: runID, ParentJobID: caller.ID}

		got, err := getScopedSecretsForJob(ctx, leaf, base)
		require.NoError(t, err)
		_, present := got["MAPPED_ALIAS"]
		assert.False(t, present)
	})

	t.Run("Nested_InheritThenInherit_FullScope", func(t *testing.T) {
		const runID = 9007
		outer := insertCaller(t, runID, 0, "inherit")
		inner := insertCaller(t, runID, outer.ID, "inherit")
		leaf := &actions_model.ActionRunJob{RunID: runID, ParentJobID: inner.ID}

		got, err := getScopedSecretsForJob(ctx, leaf, base)
		require.NoError(t, err)
		assert.Equal(t, base, got, "inherit-then-inherit should pass the full base scope through")
	})

	t.Run("Nested_InheritThenMapping_InnerNarrows", func(t *testing.T) {
		const runID = 9008
		// inner mapping narrows the full scope it inherited from outer.
		outer := insertCaller(t, runID, 0, "inherit")
		inner := insertCaller(t, runID, outer.ID, `{"ALIAS_OUT":"PROD_API_KEY"}`)
		leaf := &actions_model.ActionRunJob{RunID: runID, ParentJobID: inner.ID}

		got, err := getScopedSecretsForJob(ctx, leaf, base)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"GITHUB_TOKEN": "tok",
			"GITEA_TOKEN":  "tok",
			"ALIAS_OUT":    "prod-secret",
			// no "dev-secret"
		}, got)
	})

	t.Run("Nested_MappingThenInherit_OuterNarrows", func(t *testing.T) {
		const runID = 9009
		// inner inherits outer's already-narrowed scope, so leaf sees only auto-tokens + OUTER_ALIAS.
		outer := insertCaller(t, runID, 0, `{"OUTER_ALIAS":"PROD_API_KEY"}`)
		inner := insertCaller(t, runID, outer.ID, "inherit")
		leaf := &actions_model.ActionRunJob{RunID: runID, ParentJobID: inner.ID}

		got, err := getScopedSecretsForJob(ctx, leaf, base)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"GITHUB_TOKEN": "tok",
			"GITEA_TOKEN":  "tok",
			"OUTER_ALIAS":  "prod-secret",
			// no "dev-secret"
		}, got)
	})

	t.Run("Nested_MappingThenMapping_InnerSourceMustExistInOuterScope", func(t *testing.T) {
		const runID = 9010
		// inner can rename ALIAS_A (in outer's scope) to ALIAS_C, but cannot forward DEV_API_KEY, which outer dropped.
		outer := insertCaller(t, runID, 0, `{"ALIAS_A":"PROD_API_KEY"}`)
		inner := insertCaller(t, runID, outer.ID, `{"ALIAS_B":"DEV_API_KEY","ALIAS_C":"ALIAS_A"}`)
		leaf := &actions_model.ActionRunJob{RunID: runID, ParentJobID: inner.ID}

		got, err := getScopedSecretsForJob(ctx, leaf, base)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{
			"GITHUB_TOKEN": "tok",
			"GITEA_TOKEN":  "tok",
			"ALIAS_C":      "prod-secret",
			// no "dev-secret"
		}, got)
	})
}
