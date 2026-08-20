// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"slices"
	"testing"

	"gitea.dev/modelmigration/migrationtest"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm/schemas"
)

type secretBeforeV349 struct {
	ID          int64              `xorm:"pk autoincr"`
	OwnerID     int64              `xorm:"INDEX UNIQUE(owner_repo_name) NOT NULL"`
	RepoID      int64              `xorm:"INDEX UNIQUE(owner_repo_name) NOT NULL DEFAULT 0"`
	Name        string             `xorm:"UNIQUE(owner_repo_name) NOT NULL"`
	Data        string             `xorm:"LONGTEXT"`
	Description string             `xorm:"TEXT"`
	CreatedUnix timeutil.TimeStamp `xorm:"created NOT NULL"`
}

func (secretBeforeV349) TableName() string { return "secret" }

type actionVariableBeforeV349 struct {
	ID          int64              `xorm:"pk autoincr"`
	OwnerID     int64              `xorm:"UNIQUE(owner_repo_name)"`
	RepoID      int64              `xorm:"INDEX UNIQUE(owner_repo_name)"`
	Name        string             `xorm:"UNIQUE(owner_repo_name) NOT NULL"`
	Data        string             `xorm:"LONGTEXT NOT NULL"`
	Description string             `xorm:"TEXT"`
	CreatedUnix timeutil.TimeStamp `xorm:"created NOT NULL"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
}

func (actionVariableBeforeV349) TableName() string { return "action_variable" }

type actionRunJobBeforeV349 struct {
	ID     int64 `xorm:"pk autoincr"`
	RepoID int64 `xorm:"index"`
}

func (actionRunJobBeforeV349) TableName() string { return "action_run_job" }

type secretV349 struct {
	ID            int64              `xorm:"pk autoincr"`
	OwnerID       int64              `xorm:"INDEX UNIQUE(owner_repo_name) NOT NULL"`
	RepoID        int64              `xorm:"INDEX UNIQUE(owner_repo_name) NOT NULL DEFAULT 0"`
	EnvironmentID int64              `xorm:"INDEX UNIQUE(owner_repo_name) NOT NULL DEFAULT 0"`
	Name          string             `xorm:"UNIQUE(owner_repo_name) NOT NULL"`
	Data          string             `xorm:"LONGTEXT"`
	CreatedUnix   timeutil.TimeStamp `xorm:"created NOT NULL"`
}

func (secretV349) TableName() string { return "secret" }

func Test_AddActionEnvironmentSchema(t *testing.T) {
	x, deferable := migrationtest.PrepareTestEnv(t, 0,
		new(secretBeforeV349),
		new(actionVariableBeforeV349),
		new(actionRunJobBeforeV349),
	)
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	_, err := x.Insert(&secretBeforeV349{OwnerID: 1, Name: "TOKEN", Data: "secret-data"})
	require.NoError(t, err)

	require.NoError(t, AddActionEnvironmentSchema(t.Context(), x))

	tableMap := migrationtest.LoadTableSchemasMap(t, x)
	require.ElementsMatch(t, tableMap["action_environment"].ColumnsSeq(),
		[]string{"id", "repo_id", "name", "lower_name", "allowed_branch_patterns", "created_unix", "updated_unix"})
	require.Contains(t, tableMap["action_run_job"].ColumnsSeq(), "environment_name")

	// adding a column must not cost the table its indices: Sync drops every index a partial struct omits
	jobIndexes, err := x.Dialect().GetIndexes(x.DB(), t.Context(), "action_run_job")
	require.NoError(t, err)
	assert.Len(t, jobIndexes, 1, "the pre-existing repo_id index must survive")

	// pre-existing rows survive the recreate and land in the repo/org scope
	migrated := &secretV349{}
	has, err := x.Where("owner_id = ? AND name = ?", 1, "TOKEN").Get(migrated)
	require.NoError(t, err)
	require.True(t, has)
	assert.EqualValues(t, 0, migrated.EnvironmentID)
	assert.Equal(t, "secret-data", migrated.Data)

	for _, table := range []string{"secret", "action_variable"} {
		indexes, err := x.Dialect().GetIndexes(x.DB(), t.Context(), table)
		require.NoError(t, err)
		assert.True(t, hasUniqueIndexOn(indexes, []string{"owner_id", "repo_id", "environment_id", "name"}),
			"%s must scope its unique name constraint by environment", table)
	}

	// the same name is reusable across environments, but not within one
	_, err = x.Insert(&secretV349{OwnerID: 1, EnvironmentID: 1, Name: "TOKEN", Data: "env-scoped"})
	require.NoError(t, err)
	_, err = x.Insert(&secretV349{OwnerID: 1, EnvironmentID: 1, Name: "TOKEN", Data: "dup"})
	require.Error(t, err)
}

func hasUniqueIndexOn(indexes map[string]*schemas.Index, cols []string) bool {
	for _, index := range indexes {
		if index.Type == schemas.UniqueType && slices.Equal(index.Cols, cols) {
			return true
		}
	}
	return false
}
