// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddImmutableReleases(t *testing.T) {
	type Release struct {
		ID           int64 `xorm:"pk autoincr"`
		LowerTagName string
	}

	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(Release))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	require.NoError(t, AddImmutableReleases(t.Context(), x))

	tables := migrationtest.LoadTableSchemasMap(t, x)
	assert.NotNil(t, tables["immutable_tag"])
	assert.NotNil(t, tables["release"].GetColumn("is_immutable"))
	assert.Nil(t, tables["release"].GetColumn("lower_tag_name"))
}
