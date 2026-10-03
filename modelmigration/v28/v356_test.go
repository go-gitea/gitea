// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddActionRunCommitSHAIndex(t *testing.T) {
	type ActionRun struct {
		ID        int64 `xorm:"pk autoincr"`
		CommitSHA string
	}

	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(ActionRun))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	require.NoError(t, AddActionRunCommitSHAIndex(t.Context(), x))

	tables := migrationtest.LoadTableSchemasMap(t, x)
	schema, ok := tables["action_run"]
	require.True(t, ok)
	var cols [][]string
	for _, idx := range schema.Indexes {
		cols = append(cols, idx.Cols)
	}
	assert.Contains(t, cols, []string{"commit_sha"})
}
