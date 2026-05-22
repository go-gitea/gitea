// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"

	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
)

func TestDefaultTargetBranchSelection(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	ctx := t.Context()
	repo := unittest.AssertExistsAndLoadBean(t, &Repository{ID: 1})

	assert.Equal(t, repo.DefaultBranch, repo.GetPullRequestTargetBranch(ctx))

	repo.Units = nil
	prUnit, err := repo.GetUnit(ctx, unit.TypePullRequests)
	assert.NoError(t, err)
	prConfig := prUnit.PullRequestsConfig()
	prConfig.DefaultTargetBranch = "branch2"
	prUnit.Config = prConfig
	assert.NoError(t, UpdateRepoUnitConfig(ctx, prUnit))
	repo.Units = nil
	assert.Equal(t, "branch2", repo.GetPullRequestTargetBranch(ctx))
}

func TestPullRequestConfigFromDB(t *testing.T) {
	cases := []struct {
		// name describes the row shape under test; the comments capture why each row matters.
		name              string
		json              string
		wantMergeUpdate   bool
		wantRebaseUpdate  bool
		wantDefaultStyle  UpdateStyle
		wantValidatesPass bool
	}{
		{
			// Empty object exercises the all-defaults path (e.g. fresh repos created via low-level paths).
			name: "defaults", json: "{}",
			wantMergeUpdate: true, wantRebaseUpdate: true,
			wantDefaultStyle: UpdateStyleMerge, wantValidatesPass: true,
		},
		{
			// Realistic upgrade case: pre-PR JSON lacks the new fields and has AllowRebaseUpdate=false.
			// Historical setting must be preserved while new fields take safe defaults.
			name:            "legacy without new fields",
			json:            `{"AllowMerge":true,"AllowRebase":true,"AllowRebaseMerge":true,"AllowSquash":true,"AllowRebaseUpdate":false}`,
			wantMergeUpdate: true, wantRebaseUpdate: false,
			wantDefaultStyle: UpdateStyleMerge, wantValidatesPass: true,
		},
		{
			// Partially-migrated row with explicit empty string must normalize so ValidateUpdateSettings passes.
			name: "empty default style", json: `{"DefaultUpdateStyle":""}`,
			wantMergeUpdate: true, wantRebaseUpdate: true,
			wantDefaultStyle: UpdateStyleMerge, wantValidatesPass: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := new(PullRequestsConfig)
			assert.NoError(t, cfg.FromDB([]byte(tc.json)))
			assert.Equal(t, tc.wantMergeUpdate, cfg.AllowMergeUpdate)
			assert.Equal(t, tc.wantRebaseUpdate, cfg.AllowRebaseUpdate)
			assert.Equal(t, tc.wantDefaultStyle, cfg.DefaultUpdateStyle)
			if tc.wantValidatesPass {
				assert.NoError(t, cfg.ValidateUpdateSettings())
			}
		})
	}
}

func TestPullRequestConfigValidateUpdateSettingsInvalidArgument(t *testing.T) {
	cases := []struct {
		name string
		cfg  PullRequestsConfig
	}{
		{
			name: "invalid default style",
			cfg: PullRequestsConfig{
				AllowMergeUpdate:   true,
				AllowRebaseUpdate:  true,
				DefaultUpdateStyle: "invalid",
			},
		},
		{
			name: "no update style enabled",
			cfg: PullRequestsConfig{
				DefaultUpdateStyle: UpdateStyleMerge,
			},
		},
		{
			name: "default update style disabled",
			cfg: PullRequestsConfig{
				AllowRebaseUpdate:  true,
				DefaultUpdateStyle: UpdateStyleMerge,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.ErrorIs(t, tc.cfg.ValidateUpdateSettings(), util.ErrInvalidArgument)
		})
	}
}
