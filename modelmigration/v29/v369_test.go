// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"
	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/require"
)

func TestDropLegacyActionRunConcurrencyColumns(t *testing.T) {
	if !setting.Database.Type.IsSQLite3() {
		t.Skip("SQLite-only migration")
	}

	x, deferable := migrationtest.PrepareTestEnv(t, 0)
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	_, err := x.Exec("CREATE TABLE `action_run` (`id` INTEGER PRIMARY KEY, \"concurrency_group\" TEXT NOT NULL, \"concurrency_cancel\" INTEGER NOT NULL)")
	require.NoError(t, err)

	require.NoError(t, DropLegacyActionRunConcurrencyColumns(t.Context(), x))

	_, err = x.Exec(`INSERT INTO "action_run" ("id") VALUES (1)`)
	require.NoError(t, err)

	require.NoError(t, DropLegacyActionRunConcurrencyColumns(t.Context(), x))
}
