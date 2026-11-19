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

	for _, testCase := range []struct {
		name        string
		runID       int64
		callSecrets []string
		check       func(*testing.T, map[string]string)
	}{
		{"TopLevelJob_ReturnsBaseUnchanged", 9001, nil, func(t *testing.T, got map[string]string) {
			assert.Equal(t, base, got, "top-level jobs should see the full base scope")
		}},
		{"CallerInherit_PassesParentScopeThrough", 9002, []string{"inherit"}, func(t *testing.T, got map[string]string) {
			assert.Equal(t, base, got, "secrets: inherit forwards everything from parent scope")
		}},
		{"CallerEmptySecrets_ExposesOnlyAutoTokens", 9003, []string{""}, func(t *testing.T, got map[string]string) {
			assert.Equal(t, map[string]string{
				"GITHUB_TOKEN": "tok",
				"GITEA_TOKEN":  "tok",
			}, got)
		}},
		// {alias: source} - the called workflow sees `secrets.MY_KEY` resolved to PROD_API_KEY's value.
		{"CallerMapping_OnlyMappedAliasesPlusTokens", 9004, []string{`{"MY_KEY":"PROD_API_KEY"}`}, func(t *testing.T, got map[string]string) {
			assert.Equal(t, map[string]string{
				"GITHUB_TOKEN": "tok",
				"GITEA_TOKEN":  "tok",
				"MY_KEY":       "prod-secret",
				// no "dev-secret"
			}, got)
		}},
		{"CallerMapping_CaseInsensitiveSource", 9005, []string{`{"alias":"prod_api_key"}`}, func(t *testing.T, got map[string]string) {
			assert.Equal(t, "prod-secret", got["alias"])
		}},
		// alias points at a non-existent secret name, so it must be dropped.
		{"CallerMapping_UnknownSourceDropsAlias", 9006, []string{`{"MAPPED_ALIAS":"DOES_NOT_EXIST"}`}, func(t *testing.T, got map[string]string) {
			_, present := got["MAPPED_ALIAS"]
			assert.False(t, present)
		}},
		{"Nested_InheritThenInherit_FullScope", 9007, []string{"inherit", "inherit"}, func(t *testing.T, got map[string]string) {
			assert.Equal(t, base, got, "inherit-then-inherit should pass the full base scope through")
		}},
		// inner mapping narrows the full scope it inherited from outer.
		{"Nested_InheritThenMapping_InnerNarrows", 9008, []string{"inherit", `{"ALIAS_OUT":"PROD_API_KEY"}`}, func(t *testing.T, got map[string]string) {
			assert.Equal(t, map[string]string{
				"GITHUB_TOKEN": "tok",
				"GITEA_TOKEN":  "tok",
				"ALIAS_OUT":    "prod-secret",
				// no "dev-secret"
			}, got)
		}},
		// inner inherits outer's already-narrowed scope, so leaf sees only auto-tokens + OUTER_ALIAS.
		{"Nested_MappingThenInherit_OuterNarrows", 9009, []string{`{"OUTER_ALIAS":"PROD_API_KEY"}`, "inherit"}, func(t *testing.T, got map[string]string) {
			assert.Equal(t, map[string]string{
				"GITHUB_TOKEN": "tok",
				"GITEA_TOKEN":  "tok",
				"OUTER_ALIAS":  "prod-secret",
				// no "dev-secret"
			}, got)
		}},
		// inner can rename ALIAS_A (in outer's scope) to ALIAS_C, but cannot forward DEV_API_KEY, which outer dropped.
		{"Nested_MappingThenMapping_InnerSourceMustExistInOuterScope", 9010, []string{`{"ALIAS_A":"PROD_API_KEY"}`, `{"ALIAS_B":"DEV_API_KEY","ALIAS_C":"ALIAS_A"}`}, func(t *testing.T, got map[string]string) {
			assert.Equal(t, map[string]string{
				"GITHUB_TOKEN": "tok",
				"GITEA_TOKEN":  "tok",
				"ALIAS_C":      "prod-secret",
				// no "dev-secret"
			}, got)
		}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			leaf := &actions_model.ActionRunJob{RunID: testCase.runID, ParentJobID: 0}
			for _, callSecrets := range testCase.callSecrets {
				leaf.ParentJobID = insertCaller(t, testCase.runID, leaf.ParentJobID, callSecrets).ID
			}
			got, err := getScopedSecretsForJob(ctx, leaf, base)
			require.NoError(t, err)
			testCase.check(t, got)
		})
	}
}
