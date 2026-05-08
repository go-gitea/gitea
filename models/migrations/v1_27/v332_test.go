// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/stretchr/testify/require"
	"xorm.io/xorm"
	"xorm.io/xorm/names"
)

func Test_AddCancellingSupportToActionRunner(t *testing.T) {
	type ActionRunner struct {
		ID   int64 `xorm:"pk autoincr"`
		Name string
	}

	x, err := xorm.NewEngine("sqlite3", filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	defer func() {
		require.NoError(t, x.Close())
	}()
	x.SetMapper(names.GonicMapper{})

	require.NoError(t, x.Sync(new(ActionRunner)))

	_, err = x.Insert(&ActionRunner{Name: "runner"})
	require.NoError(t, err)

	require.NoError(t, AddCancellingSupportToActionRunner(x))

	var hasCancellingSupport bool
	has, err := x.SQL("SELECT has_cancelling_support FROM action_runner WHERE id = ?", 1).Get(&hasCancellingSupport)
	require.NoError(t, err)
	require.True(t, has)
	require.False(t, hasCancellingSupport)
}
