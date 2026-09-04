// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type protectedBranchBeforeV353 struct {
	ID         int64  `xorm:"pk autoincr"`
	RepoID     int64  `xorm:"UNIQUE(s)"`
	BranchName string `xorm:"UNIQUE(s)"`
}

func (protectedBranchBeforeV353) TableName() string { return "protected_branch" }

func TestAddDeletionAllowlistToBranchProtection(t *testing.T) {
	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(protectedBranchBeforeV353))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	_, err := x.Insert(&protectedBranchBeforeV353{RepoID: 1, BranchName: "release/*"})
	require.NoError(t, err)
	require.NoError(t, AddDeletionAllowlistToBranchProtection(t.Context(), x))

	type protectedBranchAfterV353 struct {
		CanDelete                bool
		EnableDeletionAllowlist  bool
		DeletionAllowlistUserIDs []int64 `xorm:"JSON TEXT"`
		DeletionAllowlistTeamIDs []int64 `xorm:"JSON TEXT"`
	}

	var branch protectedBranchAfterV353
	has, err := x.Table("protected_branch").Where("repo_id = ? AND branch_name = ?", 1, "release/*").Get(&branch)
	require.NoError(t, err)
	require.True(t, has)
	assert.False(t, branch.CanDelete)
	assert.False(t, branch.EnableDeletionAllowlist)
	assert.Nil(t, branch.DeletionAllowlistUserIDs)
	assert.Nil(t, branch.DeletionAllowlistTeamIDs)
}
