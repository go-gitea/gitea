// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/require"
)

type actionRunWithLegacyConcurrency struct {
	ID                int64 `xorm:"pk autoincr"`
	RepoID            int64
	ConcurrencyGroup  string `xorm:"NOT NULL DEFAULT ''"`
	ConcurrencyCancel bool   `xorm:"NOT NULL DEFAULT FALSE"`
}

func (actionRunWithLegacyConcurrency) TableName() string {
	return "action_run"
}

func TestDropLegacyActionRunConcurrencyColumns(t *testing.T) {
	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(actionRunWithLegacyConcurrency))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	_, err := x.Insert(&actionRunWithLegacyConcurrency{RepoID: 1, ConcurrencyGroup: "group"})
	require.NoError(t, err)

	require.NoError(t, DropLegacyActionRunConcurrencyColumns(t.Context(), x))

	for _, name := range []string{"concurrency_group", "concurrency_cancel"} {
		exist, err := x.Dialect().IsColumnExist(x.DB(), t.Context(), "action_run", name)
		require.NoError(t, err)
		require.False(t, exist, name)
	}

	count, err := x.Table("action_run").Count()
	require.NoError(t, err)
	require.EqualValues(t, 1, count)

	require.NoError(t, DropLegacyActionRunConcurrencyColumns(t.Context(), x))
}
