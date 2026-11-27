// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"slices"
	"testing"

	"gitea.dev/modelmigration/migrationtest"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm/schemas"
)

type secretBeforeV376 struct {
	ID          int64              `xorm:"pk autoincr"`
	OwnerID     int64              `xorm:"INDEX UNIQUE(owner_repo_name) NOT NULL"`
	RepoID      int64              `xorm:"INDEX UNIQUE(owner_repo_name) NOT NULL DEFAULT 0"`
	Name        string             `xorm:"UNIQUE(owner_repo_name) NOT NULL"`
	Data        string             `xorm:"LONGTEXT"`
	Description string             `xorm:"TEXT"`
	CreatedUnix timeutil.TimeStamp `xorm:"created NOT NULL"`
}

func (secretBeforeV376) TableName() string { return "secret" }

type actionVariableBeforeV376 struct {
	ID          int64              `xorm:"pk autoincr"`
	OwnerID     int64              `xorm:"UNIQUE(owner_repo_name)"`
	RepoID      int64              `xorm:"INDEX UNIQUE(owner_repo_name)"`
	Name        string             `xorm:"UNIQUE(owner_repo_name) NOT NULL"`
	Data        string             `xorm:"LONGTEXT NOT NULL"`
	Description string             `xorm:"TEXT"`
	CreatedUnix timeutil.TimeStamp `xorm:"created NOT NULL"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
}

func (actionVariableBeforeV376) TableName() string { return "action_variable" }

type actionRunJobBeforeV376 struct {
	ID     int64 `xorm:"pk autoincr"`
	RepoID int64 `xorm:"index"`
}

func (actionRunJobBeforeV376) TableName() string { return "action_run_job" }

type secretV376 struct {
	ID            int64              `xorm:"pk autoincr"`
	OwnerID       int64              `xorm:"INDEX UNIQUE(owner_repo_name) NOT NULL"`
	RepoID        int64              `xorm:"INDEX UNIQUE(owner_repo_name) NOT NULL DEFAULT 0"`
	EnvironmentID int64              `xorm:"INDEX UNIQUE(owner_repo_name) NOT NULL DEFAULT 0"`
	Name          string             `xorm:"UNIQUE(owner_repo_name) NOT NULL"`
	Data          string             `xorm:"LONGTEXT"`
	CreatedUnix   timeutil.TimeStamp `xorm:"created NOT NULL"`
}

func (secretV376) TableName() string { return "secret" }

func Test_AddActionEnvironmentSchema(t *testing.T) {
	x, deferable := migrationtest.PrepareTestEnv(t, 0,
		new(secretBeforeV376),
		new(actionVariableBeforeV376),
		new(actionRunJobBeforeV376),
	)
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	_, err := x.Insert(&secretBeforeV376{OwnerID: 1, Name: "TOKEN", Data: "secret-data"})
	require.NoError(t, err)

	require.NoError(t, AddActionEnvironmentSchema(t.Context(), x))

	jobIndexes, err := x.Dialect().GetIndexes(x.DB(), t.Context(), "action_run_job")
	require.NoError(t, err)
	assert.Len(t, jobIndexes, 1, "the pre-existing repo_id index must survive")

	migrated := &secretV376{}
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
}

func hasUniqueIndexOn(indexes map[string]*schemas.Index, cols []string) bool {
	for _, index := range indexes {
		if index.Type == schemas.UniqueType && slices.Equal(index.Cols, cols) {
			return true
		}
	}
	return false
}
