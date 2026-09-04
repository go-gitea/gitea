// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

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

	type ImmutableTag struct {
		ID             int64  `xorm:"pk autoincr"`
		LowerOwnerName string `xorm:"UNIQUE(s) NOT NULL"`
		LowerRepoName  string `xorm:"UNIQUE(s) NOT NULL"`
		TagName        string `xorm:"UNIQUE(s) NOT NULL"`
	}

	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(Release))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}
	_, err := x.Insert(&Release{LowerTagName: "v1.0"})
	require.NoError(t, err)

	require.NoError(t, AddImmutableReleases(t.Context(), x))

	_, err = x.Insert(&ImmutableTag{LowerOwnerName: "o", LowerRepoName: "r", TagName: "v1.0"})
	require.NoError(t, err)

	_, err = x.Insert(&ImmutableTag{LowerOwnerName: "o", LowerRepoName: "r", TagName: "v1.0"})
	assert.Error(t, err)

	_, err = x.Insert(&ImmutableTag{LowerOwnerName: "o", LowerRepoName: "r2", TagName: "v1.0"})
	assert.NoError(t, err)

	release := migrationtest.LoadTableSchemasMap(t, x)["release"]
	require.NotNil(t, release)
	assert.NotNil(t, release.GetColumn("is_immutable"))
	assert.Nil(t, release.GetColumn("lower_tag_name"))
}
