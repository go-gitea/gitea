// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"fmt"
	"testing"

	"gitea.dev/modelmigration/migrationtest"
	"gitea.dev/modules/json"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type protectedBranchBeforeV358 struct {
	ID          int64  `xorm:"pk autoincr"`
	RepoID      int64  `xorm:"UNIQUE(s)"`
	BranchName  string `xorm:"UNIQUE(s)"`
	Priority    int64
	CreatedUnix timeutil.TimeStamp
	UpdatedUnix timeutil.TimeStamp
	Config      protectedBranchConfigV358 `xorm:"extends"`
}

func (protectedBranchBeforeV358) TableName() string { return "protected_branch" }

func TestMoveBranchProtectionConfigToJSON(t *testing.T) {
	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(protectedBranchBeforeV358))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	const configJSON = `{"can_push":true,"enable_whitelist":true,"whitelist_user_ids":[2,4],"whitelist_team_ids":[1,2],"enable_merge_whitelist":true,"whitelist_deploy_keys":true,"merge_whitelist_user_ids":[3,5],"merge_whitelist_team_ids":[3,4],"enable_bypass_allowlist":true,"bypass_allowlist_user_ids":[6,7],"bypass_allowlist_team_ids":[5,6],"can_force_push":true,"enable_force_push_allowlist":true,"force_push_allowlist_user_ids":[8,9],"force_push_allowlist_team_ids":[7,8],"force_push_allowlist_deploy_keys":true,"enable_status_check":true,"status_check_contexts":["ci/*","lint"],"enable_approvals_whitelist":true,"approvals_whitelist_user_ids":[10,11],"approvals_whitelist_team_ids":[9,10],"required_approvals":2,"block_on_rejected_reviews":true,"block_on_official_review_requests":true,"block_on_codeowner_reviews":true,"block_on_outdated_branch":true,"dismiss_stale_approvals":true,"ignore_stale_approvals":true,"require_signed_commits":true,"protected_file_patterns":"*.go;*.md","unprotected_file_patterns":"docs/**","block_admin_merge_override":true}`
	var config protectedBranchConfigV358
	require.NoError(t, json.Unmarshal([]byte(configJSON), &config))
	for i := range 101 {
		branch := protectedBranchBeforeV358{RepoID: 1, BranchName: fmt.Sprintf("release-%d/*", i), Priority: int64(i), CreatedUnix: 123, UpdatedUnix: 456}
		if i%2 == 0 {
			branch.Config = config
		} else if i == 1 {
			branch.Config.StatusCheckContexts = []string{}
		}
		_, err := x.Insert(&branch)
		require.NoError(t, err)
	}
	require.NoError(t, MoveBranchProtectionConfigToJSON(t.Context(), x))
	// A retry must preserve the migrated configuration and its indexes.
	require.NoError(t, MoveBranchProtectionConfigToJSON(t.Context(), x))

	var branches []protectedBranchV358
	require.NoError(t, x.Asc("id").Find(&branches))
	require.Len(t, branches, 101)
	for i, branch := range branches {
		assert.EqualValues(t, i+1, branch.ID)
		assert.EqualValues(t, 1, branch.RepoID)
		assert.Equal(t, fmt.Sprintf("release-%d/*", i), branch.BranchName)
		assert.EqualValues(t, i, branch.Priority)
		assert.EqualValues(t, 123, branch.CreatedUnix)
		assert.EqualValues(t, 456, branch.UpdatedUnix)
		if i%2 == 0 {
			assert.JSONEq(t, configJSON, branch.Config)
		} else if i == 1 {
			assert.JSONEq(t, `{"status_check_contexts":[]}`, branch.Config)
		} else {
			assert.JSONEq(t, "{}", branch.Config)
		}
	}
	tables, err := x.DBMetas()
	require.NoError(t, err)
	for _, table := range tables {
		if table.Name == "protected_branch" {
			assert.ElementsMatch(t, []string{"id", "repo_id", "branch_name", "priority", "config", "created_unix", "updated_unix"}, table.ColumnsSeq())
		}
	}
	_, err = x.Insert(&protectedBranchV358{RepoID: 1, BranchName: branches[0].BranchName})
	require.Error(t, err, "the repository/rule unique constraint must survive the migration")
}
