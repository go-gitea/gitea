// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/require"
)

type actionRunBeforeV349 struct {
	ID                int64  `xorm:"pk autoincr"`
	RepoID            int64  `xorm:"index(repo_concurrency)"`
	ConcurrencyGroup  string `xorm:"index(repo_concurrency) NOT NULL DEFAULT ''"`
	ConcurrencyCancel bool   `xorm:"NOT NULL DEFAULT FALSE"`
}

func (actionRunBeforeV349) TableName() string {
	return "action_run"
}

func TestDropLeftoverActionRunConcurrencyColumns(t *testing.T) {
	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(actionRunBeforeV349))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	_, err := x.Insert(&actionRunBeforeV349{RepoID: 1, ConcurrencyGroup: "group"})
	require.NoError(t, err)

	require.NoError(t, DropLeftoverActionRunConcurrencyColumns(t.Context(), x))

	for _, col := range []string{"concurrency_group", "concurrency_cancel"} {
		exist, err := x.Dialect().IsColumnExist(x.DB(), t.Context(), "action_run", col)
		require.NoError(t, err)
		require.False(t, exist, "%s must be gone", col)
	}

	count, err := x.Table("action_run").Count()
	require.NoError(t, err)
	require.EqualValues(t, 1, count, "existing runs must survive")

	require.NoError(t, DropLeftoverActionRunConcurrencyColumns(t.Context(), x), "re-running must be a no-op")
}
