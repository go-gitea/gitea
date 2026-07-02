// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_28

import (
	"context"
	"slices"
	"testing"

	"gitea.dev/models/migrations/migrationtest"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm/schemas"
)

type secretBeforeV343 struct {
	ID          int64              `xorm:"pk autoincr"`
	OwnerID     int64              `xorm:"INDEX UNIQUE(owner_repo_name) NOT NULL"`
	RepoID      int64              `xorm:"INDEX UNIQUE(owner_repo_name) NOT NULL DEFAULT 0"`
	Name        string             `xorm:"UNIQUE(owner_repo_name) NOT NULL"`
	Data        string             `xorm:"LONGTEXT"`
	Description string             `xorm:"TEXT"`
	CreatedUnix timeutil.TimeStamp `xorm:"created NOT NULL"`
}

func (secretBeforeV343) TableName() string { return "secret" }

type actionVariableBeforeV343 struct {
	ID          int64              `xorm:"pk autoincr"`
	OwnerID     int64              `xorm:"UNIQUE(owner_repo_name)"`
	RepoID      int64              `xorm:"INDEX UNIQUE(owner_repo_name)"`
	Name        string             `xorm:"UNIQUE(owner_repo_name) NOT NULL"`
	Data        string             `xorm:"LONGTEXT NOT NULL"`
	Description string             `xorm:"TEXT"`
	CreatedUnix timeutil.TimeStamp `xorm:"created NOT NULL"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
}

func (actionVariableBeforeV343) TableName() string { return "action_variable" }

type actionRunJobBeforeV343 struct {
	ID     int64 `xorm:"pk autoincr"`
	RepoID int64 `xorm:"index"`
}

func (actionRunJobBeforeV343) TableName() string { return "action_run_job" }

// secretV343 and actionVariableV343 mirror the schema after the migration so we
// can insert rows to exercise the new unique constraint that includes environment_id.
type secretV343 struct {
	ID            int64              `xorm:"pk autoincr"`
	OwnerID       int64              `xorm:"INDEX UNIQUE(owner_repo_name) NOT NULL"`
	RepoID        int64              `xorm:"INDEX UNIQUE(owner_repo_name) NOT NULL DEFAULT 0"`
	EnvironmentID int64              `xorm:"INDEX UNIQUE(owner_repo_name) NOT NULL DEFAULT 0"`
	Name          string             `xorm:"UNIQUE(owner_repo_name) NOT NULL"`
	Data          string             `xorm:"LONGTEXT"`
	CreatedUnix   timeutil.TimeStamp `xorm:"created NOT NULL"`
}

func (secretV343) TableName() string { return "secret" }

type actionVariableV343 struct {
	ID            int64              `xorm:"pk autoincr"`
	OwnerID       int64              `xorm:"UNIQUE(owner_repo_name)"`
	RepoID        int64              `xorm:"INDEX UNIQUE(owner_repo_name)"`
	EnvironmentID int64              `xorm:"INDEX UNIQUE(owner_repo_name) NOT NULL DEFAULT 0"`
	Name          string             `xorm:"UNIQUE(owner_repo_name) NOT NULL"`
	Data          string             `xorm:"LONGTEXT NOT NULL"`
	CreatedUnix   timeutil.TimeStamp `xorm:"created NOT NULL"`
}

func (actionVariableV343) TableName() string { return "action_variable" }

func Test_AddActionEnvironmentTables(t *testing.T) {
	x, deferable := migrationtest.PrepareTestEnv(t, 0,
		new(secretBeforeV343),
		new(actionVariableBeforeV343),
		new(actionRunJobBeforeV343),
	)
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	// Seed existing rows that must be preserved through the recreate.
	_, err := x.Insert(&secretBeforeV343{OwnerID: 1, RepoID: 0, Name: "TOKEN", Data: "secret-data"})
	require.NoError(t, err)
	_, err = x.Insert(&actionVariableBeforeV343{OwnerID: 1, RepoID: 0, Name: "VAR", Data: "var-data"})
	require.NoError(t, err)

	require.NoError(t, AddActionEnvironmentTables(x))

	tableMap := migrationtest.LoadTableSchemasMap(t, x)

	// action_environment table is created with the expected columns.
	envTable := tableMap["action_environment"]
	require.NotNil(t, envTable)
	require.ElementsMatch(t, envTable.ColumnsSeq(),
		[]string{"id", "repo_id", "name", "protected_branches", "created_unix", "updated_unix"})

	// environment_id is added to secret and action_variable.
	secretTable := tableMap["secret"]
	require.NotNil(t, secretTable)
	require.Contains(t, secretTable.ColumnsSeq(), "environment_id")

	variableTable := tableMap["action_variable"]
	require.NotNil(t, variableTable)
	require.Contains(t, variableTable.ColumnsSeq(), "environment_id")

	// environment_name is added to action_run_job.
	jobTable := tableMap["action_run_job"]
	require.NotNil(t, jobTable)
	require.Contains(t, jobTable.ColumnsSeq(), "environment_name")

	// Existing data is preserved and back-filled with environment_id 0.
	migratedSecret := &secretV343{}
	has, err := x.Where("owner_id = ? AND repo_id = ? AND name = ?", 1, 0, "TOKEN").Get(migratedSecret)
	require.NoError(t, err)
	require.True(t, has)
	assert.EqualValues(t, 0, migratedSecret.EnvironmentID)
	assert.Equal(t, "secret-data", migratedSecret.Data)

	// The unique constraint now includes environment_id.
	secretIndexes, err := x.Dialect().GetIndexes(x.DB(), context.Background(), "secret")
	require.NoError(t, err)
	assert.True(t, hasIndexWithColumns(secretIndexes, []string{"owner_id", "repo_id", "environment_id", "name"}, true))

	variableIndexes, err := x.Dialect().GetIndexes(x.DB(), context.Background(), "action_variable")
	require.NoError(t, err)
	assert.True(t, hasIndexWithColumns(variableIndexes, []string{"owner_id", "repo_id", "environment_id", "name"}, true))

	// Same owner/repo/name but a different environment is allowed.
	_, err = x.Insert(&secretV343{OwnerID: 1, RepoID: 0, EnvironmentID: 1, Name: "TOKEN", Data: "env-scoped"})
	require.NoError(t, err)

	// Duplicate owner/repo/environment/name is rejected.
	_, err = x.Insert(&secretV343{OwnerID: 1, RepoID: 0, EnvironmentID: 1, Name: "TOKEN", Data: "dup"})
	require.Error(t, err)

	_, err = x.Insert(&actionVariableV343{OwnerID: 1, RepoID: 0, EnvironmentID: 1, Name: "VAR", Data: "env-scoped"})
	require.NoError(t, err)
	_, err = x.Insert(&actionVariableV343{OwnerID: 1, RepoID: 0, EnvironmentID: 1, Name: "VAR", Data: "dup"})
	require.Error(t, err)
}

func hasIndexWithColumns(indexes map[string]*schemas.Index, cols []string, isUnique bool) bool {
	for _, index := range indexes {
		if isUnique && index.Type != schemas.UniqueType {
			continue
		}
		if slices.Equal(index.Cols, cols) {
			return true
		}
	}
	return false
}
