// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_28

import (
	"context"
	"slices"
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm/schemas"
)

func TestAddAuditEventTable(t *testing.T) {
	x, deferable := migrationtest.PrepareTestEnv(t, 0)
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	require.NoError(t, AddAuditEventTable(x))

	indexes, err := x.Dialect().GetIndexes(x.DB(), context.Background(), "audit_event")
	require.NoError(t, err)
	for _, columns := range [][]string{
		{"action"},
		{"actor_id"},
		{"scope_id", "scope_type"},
		{"scope_type"},
		{"origin"},
		{"timestamp_unix"},
	} {
		assert.True(t, hasAuditIndexWithColumns(indexes, columns), "missing index on %v", columns)
	}
}

func hasAuditIndexWithColumns(indexes map[string]*schemas.Index, columns []string) bool {
	for _, index := range indexes {
		if slices.Equal(index.Cols, columns) {
			return true
		}
	}
	return false
}
