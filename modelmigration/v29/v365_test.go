// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddWatchRepoIDIndex(t *testing.T) {
	type Watch struct {
		ID     int64 `xorm:"pk autoincr"`
		UserID int64 `xorm:"UNIQUE(watch)"`
		RepoID int64 `xorm:"UNIQUE(watch)"`
	}

	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(Watch))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	require.NoError(t, AddWatchRepoIDIndex(t.Context(), x))

	schema, ok := migrationtest.LoadTableSchemasMap(t, x)["watch"]
	require.True(t, ok)
	var cols [][]string
	for _, idx := range schema.Indexes {
		cols = append(cols, idx.Cols)
	}
	assert.Contains(t, cols, []string{"repo_id"})
	assert.Contains(t, cols, []string{"user_id", "repo_id"})
}
