// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// repositoryBeforeV349 mirrors the pre-migration repository table: no
// storage_path column.
type repositoryBeforeV349 struct {
	ID        int64 `xorm:"pk autoincr"`
	OwnerID   int64
	OwnerName string
	LowerName string
	Name      string
}

func (repositoryBeforeV349) TableName() string { return "repository" }

func Test_AddStoragePathToRepository(t *testing.T) {
	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(repositoryBeforeV349))
	defer deferable()

	_, err := x.Insert(&repositoryBeforeV349{OwnerID: 1, OwnerName: "user2", LowerName: "repo1", Name: "repo1"})
	require.NoError(t, err)

	require.NoError(t, AddStoragePathToRepository(t.Context(), x))
	require.NoError(t, AddStoragePathToRepository(t.Context(), x)) // idempotent

	type row struct {
		StoragePath string
	}
	var rows []row
	require.NoError(t, x.SQL("SELECT storage_path FROM repository").Find(&rows))
	require.Len(t, rows, 1)
	assert.Equal(t, "", rows[0].StoragePath, "existing rows keep the legacy convention")
}
