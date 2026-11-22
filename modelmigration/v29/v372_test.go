// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"
	"testing"

	"gitea.dev/modelmigration/migrationtest"
	"gitea.dev/models/db"       //nolint:depguard // allow to access db in migration tests
	"gitea.dev/models/unittest" //nolint:depguard // allow to access db in migration tests

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm/schemas"
)

// packageVersionBeforeV372 mirrors the pre-migration package_version table:
// a 2-column UNIQUE(s) index on (package_id, lower_version) and a regular
// index on lower_version.
type packageVersionBeforeV372 struct {
	ID           int64  `xorm:"pk autoincr"`
	PackageID    int64  `xorm:"UNIQUE(s) INDEX NOT NULL"`
	Version      string `xorm:"NOT NULL"`
	LowerVersion string `xorm:"UNIQUE(s) INDEX NOT NULL"`
}

func (packageVersionBeforeV372) TableName() string { return "package_version" }

func Test_MakePackageVersionUniqueByVersion(t *testing.T) {
	x, deferrable := migrationtest.PrepareTestEnv(t, 0, new(packageVersionBeforeV372))
	defer deferrable()

	_, err := x.Insert(&packageVersionBeforeV372{PackageID: 1, Version: "v1", LowerVersion: "v1"})
	require.NoError(t, err)

	indexes, err := x.Dialect().GetIndexes(x.DB(), context.Background(), "package_version")
	require.NoError(t, err)
	oldIdx, ok := indexes["s"] // GetIndexes strips the UQE_package_version_ prefix
	require.True(t, ok, "old 2-column unique s index should exist before migration")
	assert.Equal(t, schemas.UniqueType, oldIdx.Type)
	assert.Equal(t, []string{"package_id", "lower_version"}, oldIdx.Cols)

	require.NoError(t, MakePackageVersionUniqueByVersion(t.Context(), x))
	require.NoError(t, MakePackageVersionUniqueByVersion(t.Context(), x)) // idempotent

	indexes, err = x.Dialect().GetIndexes(x.DB(), context.Background(), "package_version")
	require.NoError(t, err)
	newIdx, ok := indexes["s"] // GetIndexes strips the UQE_package_version_ prefix
	require.True(t, ok, "new unique s index must exist on (package_id, version)")
	assert.Equal(t, schemas.UniqueType, newIdx.Type)
	assert.Equal(t, []string{"package_id", "version"}, newIdx.Cols)
	lowerIdx, ok := indexes["lower_version"]
	require.True(t, ok, "regular index on lower_version must still exist")
	assert.Equal(t, schemas.IndexType, lowerIdx.Type)

	// whether versions that differ only in case can coexist depends on the database
	// collation (see MakePackageVersionUniqueByVersion), so only assert it on
	// case-sensitive ones
	result, err := db.CheckCollations(unittest.GetXORMEngine())
	require.NoError(t, err)
	if result != nil && !result.IsCollationCaseSensitive(result.DatabaseCollation) {
		t.Skip("database uses a case-insensitive collation, case-variant versions cannot coexist")
	}

	// versions of one package that differ only in case are allowed now,
	// while the exact same version is rejected
	_, err = x.Exec("INSERT INTO package_version (package_id, version, lower_version) VALUES (1, 'V1', 'v1')")
	require.NoError(t, err)
	_, err = x.Exec("INSERT INTO package_version (package_id, version, lower_version) VALUES (1, 'v1', 'v1')")
	require.Error(t, err)
	_, err = x.Exec("INSERT INTO package_version (package_id, version, lower_version) VALUES (2, 'v1', 'v1')")
	require.NoError(t, err)
}
