// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29_test

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"
	"gitea.dev/modelmigration/v29"

	"github.com/stretchr/testify/require"
)

type mirrorBeforeV362 struct {
	ID     int64 `xorm:"pk autoincr"`
	RepoID int64 `xorm:"INDEX"`
}

func (mirrorBeforeV362) TableName() string { return "mirror" }

type mirrorAfterV362 struct {
	ID              int64 `xorm:"pk autoincr"`
	RepoID          int64 `xorm:"INDEX"`
	ForcePushBackup bool  `xorm:"NOT NULL DEFAULT false"`
}

func (mirrorAfterV362) TableName() string { return "mirror" }

func TestAddBackupFieldsToMirror(t *testing.T) {
	x, cleanup := migrationtest.PrepareTestEnv(t, 0, new(mirrorBeforeV362))
	defer cleanup()
	if x == nil || t.Failed() {
		return
	}

	before := &mirrorBeforeV362{RepoID: 42}
	_, err := x.Insert(before)
	require.NoError(t, err)

	require.NoError(t, v29.AddBackupFieldsToMirror(t.Context(), x))
	var after mirrorAfterV362
	has, err := x.ID(before.ID).Get(&after)
	require.NoError(t, err)
	require.True(t, has)
	require.EqualValues(t, 42, after.RepoID)
	require.False(t, after.ForcePushBackup)

	_, err = x.ID(before.ID).Cols("force_push_backup").Update(&mirrorAfterV362{ForcePushBackup: true})
	require.NoError(t, err)
	require.NoError(t, v29.AddBackupFieldsToMirror(t.Context(), x))
	has, err = x.ID(before.ID).Get(&after)
	require.NoError(t, err)
	require.True(t, has)
	require.True(t, after.ForcePushBackup)
}
