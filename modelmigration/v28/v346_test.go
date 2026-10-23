// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"context"
	"testing"

	"gitea.dev/modelmigration/migrationtest"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm/schemas"
)

// repoLicenseBeforeV345 mirrors the pre-migration repo_license table: no
// license_path column and a 2-column UNIQUE(s) index on (repo_id, license).
type repoLicenseBeforeV345 struct {
	ID          int64 `xorm:"pk autoincr"`
	RepoID      int64 `xorm:"UNIQUE(s) NOT NULL"`
	CommitID    string
	License     string             `xorm:"VARCHAR(255) UNIQUE(s) NOT NULL"`
	CreatedUnix timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
}

func (repoLicenseBeforeV345) TableName() string { return "repo_license" }

func Test_AddLicensePathToRepoLicense(t *testing.T) {
	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(repoLicenseBeforeV345))
	defer deferable()

	_, err := x.Insert(&repoLicenseBeforeV345{RepoID: 1, CommitID: "c1", License: "MIT"})
	require.NoError(t, err)
	_, err = x.Insert(&repoLicenseBeforeV345{RepoID: 1, CommitID: "c1", License: "Apache-2.0"})
	require.NoError(t, err)
	_, err = x.Insert(&repoLicenseBeforeV345{RepoID: 2, CommitID: "c2", License: "MIT"})
	require.NoError(t, err)

	indexes, err := x.Dialect().GetIndexes(x.DB(), context.Background(), "repo_license")
	require.NoError(t, err)
	oldIdx, ok := indexes["s"] // GetIndexes strips the UQE_repo_license_ prefix
	require.True(t, ok, "old 2-column unique s index should exist before migration")
	assert.Equal(t, schemas.UniqueType, oldIdx.Type)
	assert.Equal(t, []string{"repo_id", "license"}, oldIdx.Cols)

	require.NoError(t, AddLicensePathToRepoLicense(t.Context(), x))
	require.NoError(t, AddLicensePathToRepoLicense(t.Context(), x)) // idempotent

	indexes, err = x.Dialect().GetIndexes(x.DB(), context.Background(), "repo_license")
	require.NoError(t, err)
	assert.NotContains(t, indexes, "s", "old 2-column unique s index should be gone after migration")
	newIdx, ok := indexes["path"] // GetIndexes strips the UQE_repo_license_ prefix
	require.True(t, ok, "new index must be named path (UQE_repo_license_path)")
	assert.Equal(t, schemas.UniqueType, newIdx.Type)
	assert.Equal(t, []string{"repo_id", "license", "license_path"}, newIdx.Cols)

	// pre-existing rows must default to the LICENSE path
	type licenseRow struct {
		RepoID      int64
		License     string
		LicensePath string
	}
	var rows []licenseRow
	require.NoError(t, x.SQL("SELECT repo_id, license, license_path FROM repo_license ORDER BY id").Find(&rows))
	require.Len(t, rows, 3)
	for _, r := range rows {
		assert.Equal(t, "LICENSE", r.LicensePath)
	}

	// the new index is unique per (repo_id, license, license_path): the exact
	// duplicate must be rejected, while a second path for the same license
	// in the same repo is allowed
	_, err = x.Exec("INSERT INTO repo_license (repo_id, commit_id, license, license_path) VALUES (1, 'c1', 'MIT', 'LICENSE')")
	require.Error(t, err)
	_, err = x.Exec("INSERT INTO repo_license (repo_id, commit_id, license, license_path) VALUES (1, 'c1', 'MIT', 'LICENSES/MIT.txt')")
	require.NoError(t, err)
}
