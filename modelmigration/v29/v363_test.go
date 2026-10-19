// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm/schemas"
)

type packageBeforeV363 struct {
	ID               int64  `xorm:"pk autoincr"`
	OwnerID          int64  `xorm:"UNIQUE(s) INDEX NOT NULL"`
	RepoID           int64  `xorm:"INDEX"`
	Type             string `xorm:"UNIQUE(s) INDEX NOT NULL"`
	Name             string `xorm:"NOT NULL"`
	LowerName        string `xorm:"UNIQUE(s) INDEX NOT NULL"`
	SemverCompatible bool   `xorm:"NOT NULL DEFAULT false"`
	IsInternal       bool   `xorm:"NOT NULL DEFAULT false"`
}

func (packageBeforeV363) TableName() string { return "package" }

type packageAfterV363 struct {
	ID               int64  `xorm:"pk autoincr"`
	OwnerID          int64  `xorm:"UNIQUE(s) INDEX NOT NULL"`
	TeamID           int64  `xorm:"UNIQUE(s) INDEX NOT NULL DEFAULT 0"`
	RepoID           int64  `xorm:"INDEX"`
	Type             string `xorm:"UNIQUE(s) INDEX NOT NULL"`
	Name             string `xorm:"NOT NULL"`
	LowerName        string `xorm:"UNIQUE(s) INDEX NOT NULL"`
	SemverCompatible bool   `xorm:"NOT NULL DEFAULT false"`
	IsInternal       bool   `xorm:"NOT NULL DEFAULT false"`
}

func (packageAfterV363) TableName() string { return "package" }

func Test_AddTeamIDToPackage(t *testing.T) {
	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(packageBeforeV363))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	_, err := x.Insert(&packageBeforeV363{OwnerID: 1, Type: "generic", Name: "pkg1", LowerName: "pkg1"})
	require.NoError(t, err)

	indexes, err := x.Dialect().GetIndexes(x.DB(), context.Background(), "package")
	require.NoError(t, err)
	oldIdx, ok := indexes["s"]
	require.True(t, ok, "old 3-column unique s index should exist before migration")
	assert.Equal(t, schemas.UniqueType, oldIdx.Type)
	assert.Equal(t, []string{"owner_id", "type", "lower_name"}, oldIdx.Cols)

	require.NoError(t, AddTeamIDToPackage(t.Context(), x))
	require.NoError(t, AddTeamIDToPackage(t.Context(), x)) // idempotent

	indexes, err = x.Dialect().GetIndexes(x.DB(), context.Background(), "package")
	require.NoError(t, err)
	newIdx, ok := indexes["s"]
	require.True(t, ok, "new 4-column unique s index should exist after migration")
	assert.Equal(t, schemas.UniqueType, newIdx.Type)
	assert.Equal(t, []string{"owner_id", "team_id", "type", "lower_name"}, newIdx.Cols)

	// Pre-existing rows must default to team_id = 0
	type pkgRow struct {
		ID      int64
		OwnerID int64
		TeamID  int64
	}
	var row pkgRow
	has, err := x.SQL("SELECT id, owner_id, team_id FROM package WHERE id = 1").Get(&row)
	require.NoError(t, err)
	require.True(t, has)
	assert.Equal(t, int64(0), row.TeamID)

	// Same owner and package name but different team_id should be permitted
	_, err = x.Insert(&packageAfterV363{OwnerID: 1, TeamID: 10, Type: "generic", Name: "pkg1", LowerName: "pkg1"})
	require.NoError(t, err)

	// Duplicate within the same team should be rejected
	_, err = x.Insert(&packageAfterV363{OwnerID: 1, TeamID: 10, Type: "generic", Name: "pkg1", LowerName: "pkg1"})
	require.Error(t, err)
}
