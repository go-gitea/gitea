// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/migrationtest"

	"github.com/stretchr/testify/require"
)

func Test_AddBranchProtectionBypassAllowlist(t *testing.T) {
	type ProtectedBranch struct {
		ID                     int64   `xorm:"pk autoincr"`
		RepoID                 int64   `xorm:"INDEX"`
		BranchName             string  `xorm:"INDEX"`
		EnableBypassAllowlist  bool    `xorm:"NOT NULL DEFAULT false"`
		BypassAllowlistUserIDs []int64 `xorm:"JSON TEXT"`
		BypassAllowlistTeamIDs []int64 `xorm:"JSON TEXT"`
	}

	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(ProtectedBranch))
	defer deferable()

	// Test with default values
	_, err := x.Insert(&ProtectedBranch{RepoID: 1, BranchName: "main"})
	require.NoError(t, err)

	// Test with populated allowlist
	_, err = x.Insert(&ProtectedBranch{
		RepoID:                 1,
		BranchName:             "develop",
		EnableBypassAllowlist:  true,
		BypassAllowlistUserIDs: []int64{1, 2, 3},
		BypassAllowlistTeamIDs: []int64{10, 20},
	})
	require.NoError(t, err)

	require.NoError(t, AddBranchProtectionBypassAllowlist(x))

	// Verify the default values record
	var pb ProtectedBranch
	has, err := x.Where("repo_id = ? AND branch_name = ?", 1, "main").Get(&pb)
	require.NoError(t, err)
	require.True(t, has)
	require.False(t, pb.EnableBypassAllowlist)
	require.Nil(t, pb.BypassAllowlistUserIDs)
	require.Nil(t, pb.BypassAllowlistTeamIDs)

	// Verify the populated allowlist record
	var pb2 ProtectedBranch
	has, err = x.Where("repo_id = ? AND branch_name = ?", 1, "develop").Get(&pb2)
	require.NoError(t, err)
	require.True(t, has)
	require.True(t, pb2.EnableBypassAllowlist)
	require.Equal(t, []int64{1, 2, 3}, pb2.BypassAllowlistUserIDs)
	require.Equal(t, []int64{10, 20}, pb2.BypassAllowlistTeamIDs)
}
