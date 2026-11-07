// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/require"
)

func TestDropLegacyActionRunConcurrencyColumns(t *testing.T) {
	x, deferable := migrationtest.PrepareTestEnv(t, 0)
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	// schema left behind by migration 331 on SQLite: the legacy NOT NULL columns survive because the stored text did not use backticks
	_, err := x.Exec(`CREATE TABLE "action_run" ("id" INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL, "repo_id" INTEGER NULL, "concurrency_group" TEXT NOT NULL, "concurrency_cancel" BOOLEAN NOT NULL)`)
	require.NoError(t, err)
	_, err = x.Exec("CREATE INDEX `IDX_action_run_repo_concurrency` ON `action_run` (`repo_id`, `concurrency_group`)")
	require.NoError(t, err)
	_, err = x.Exec(`INSERT INTO "action_run" ("repo_id", "concurrency_group", "concurrency_cancel") VALUES (1, 'group', 0)`)
	require.NoError(t, err)

	// the symptom from the issue: no model writes the leftover columns any more, so inserting a run fails
	_, err = x.Exec(`INSERT INTO "action_run" ("repo_id") VALUES (2)`)
	require.ErrorContains(t, err, "NOT NULL constraint failed: action_run.concurrency_group")

	require.NoError(t, DropLegacyActionRunConcurrencyColumns(t.Context(), x))

	for _, name := range []string{"concurrency_group", "concurrency_cancel"} {
		exist, err := x.Dialect().IsColumnExist(x.DB(), t.Context(), "action_run", name)
		require.NoError(t, err)
		require.False(t, exist, name)
	}

	_, err = x.Exec(`INSERT INTO "action_run" ("repo_id") VALUES (2)`)
	require.NoError(t, err)

	count, err := x.Table("action_run").Count()
	require.NoError(t, err)
	require.EqualValues(t, 2, count)

	require.NoError(t, DropLegacyActionRunConcurrencyColumns(t.Context(), x))
}
