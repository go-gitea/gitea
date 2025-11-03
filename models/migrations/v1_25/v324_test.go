// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_25

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/base"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

func prepareOldProtectedBranch(t *testing.T) (*xorm.Engine, func()) {
	type ProtectedBranch struct {
		ID       int64  `xorm:"pk autoincr"`
		RepoID   int64  `xorm:"UNIQUE(s)"`
		RuleName string `xorm:"'branch_name' UNIQUE(s)"`
	}

	return base.PrepareTestEnv(t, 0, new(ProtectedBranch))
}

func Test_AddOwnerIDToProtectedBranch(t *testing.T) {
	x, deferable := prepareOldProtectedBranch(t)
	defer deferable()
	if x == nil {
		return
	}

	// The test fixtures will have already created the UQE_protected_branch_s index.
	// We can run the migration.
	assert.NoError(t, AddOwnerIDToProtectedBranch(x))

	// Verify that the new column exists.
	type ProtectedBranch struct {
		ID       int64 `xorm:"pk autoincr"`
		RepoID   int64 `xorm:"INDEX DEFAULT 0"`
		OwnerID  int64 `xorm:"INDEX DEFAULT 0"`
		RuleName string
	}

	has, err := x.Dialect().IsColumnExist(x.DB(), t.Context(), "protected_branch", "owner_id")
	assert.NoError(t, err)
	assert.True(t, has, "owner_id column should exist")

	// Skip index check for unsupported DBs
	if setting.Database.Type.IsMSSQL() || setting.Database.Type.IsSQLite3() {
		t.Log("Skipping index check for unsupported database")
		return
	}

	table, err := x.TableInfo("protected_branch")
	assert.NoError(t, err)

	// Check if the old index is gone
	_, exist := table.Indexes["UQE_protected_branch_s"]
	assert.False(t, exist, "old index UQE_protected_branch_s should not exist")

	// Check if new indexes are created
	idx, exist := table.Indexes["UQE_protected_branch_repo_id_branch_name"]
	assert.True(t, exist, "new index UQE_protected_branch_repo_id_branch_name should exist")
	if exist {
		assert.Equal(t, schemas.UniqueType, idx.Type)
		assert.ElementsMatch(t, []string{"repo_id", "branch_name"}, idx.Cols)
	}

	idx, exist = table.Indexes["UQE_protected_branch_owner_id_branch_name"]
	assert.True(t, exist, "new index UQE_protected_branch_owner_id_branch_name should exist")
	if exist {
		assert.Equal(t, schemas.UniqueType, idx.Type)
		assert.ElementsMatch(t, []string{"owner_id", "branch_name"}, idx.Cols)
	}

	// Test repo-level unique constraint
	_, err = x.Insert(&ProtectedBranch{RepoID: 1, RuleName: "main"})
	assert.NoError(t, err)
	_, err = x.Insert(&ProtectedBranch{RepoID: 1, RuleName: "main"})
	assert.Error(t, err, "should fail to insert duplicate repo-level rule")

	// Test org-level unique constraint
	_, err = x.Insert(&ProtectedBranch{OwnerID: 1, RuleName: "main"})
	assert.NoError(t, err)
	_, err = x.Insert(&ProtectedBranch{OwnerID: 1, RuleName: "main"})
	assert.Error(t, err, "should fail to insert duplicate org-level rule")

	// Test that repo-level and org-level rules with the same name don't conflict
	_, err = x.Insert(&ProtectedBranch{RepoID: 2, RuleName: "develop"})
	assert.NoError(t, err)
	_, err = x.Insert(&ProtectedBranch{OwnerID: 2, RuleName: "develop"})
	assert.NoError(t, err)

	// Test that rules with repo_id=0 or owner_id=0 don't conflict with partial indexes
	_, err = x.Insert(&ProtectedBranch{RepoID: 3, OwnerID: 0, RuleName: "feature-a"})
	assert.NoError(t, err)
	_, err = x.Insert(&ProtectedBranch{RepoID: 4, OwnerID: 0, RuleName: "feature-a"})
	assert.NoError(t, err)

	_, err = x.Insert(&ProtectedBranch{RepoID: 0, OwnerID: 3, RuleName: "feature-b"})
	assert.NoError(t, err)
	_, err = x.Insert(&ProtectedBranch{RepoID: 0, OwnerID: 4, RuleName: "feature-b"})
	assert.NoError(t, err)
}
