// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddPackagePropertyRefIndex(t *testing.T) {
	type PackageProperty struct {
		ID      int64 `xorm:"pk autoincr"`
		RefType int64 `xorm:"INDEX NOT NULL"`
		RefID   int64 `xorm:"INDEX NOT NULL"`
	}

	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(PackageProperty))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	require.NoError(t, AddPackagePropertyRefIndex(t.Context(), x))

	var cols [][]string
	for _, idx := range migrationtest.LoadTableSchemasMap(t, x)["package_property"].Indexes {
		cols = append(cols, idx.Cols)
	}
	assert.Contains(t, cols, []string{"ref_type", "ref_id"})
	assert.Contains(t, cols, []string{"ref_type"})
}
