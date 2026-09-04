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
	type ImmutableTag struct {
		ID             int64  `xorm:"pk autoincr"`
		LowerOwnerName string `xorm:"UNIQUE(s) NOT NULL"`
		LowerRepoName  string `xorm:"UNIQUE(s) NOT NULL"`
		TagName        string `xorm:"UNIQUE(s) NOT NULL"`
	}

	x, deferable := migrationtest.PrepareTestEnv(t, 0)
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	require.NoError(t, AddImmutableReleases(t.Context(), x))

	_, err := x.Insert(&ImmutableTag{LowerOwnerName: "o", LowerRepoName: "r", TagName: "v1.0"})
	require.NoError(t, err)

	// the unique index must be created by the migration, not only on fresh installs
	_, err = x.Insert(&ImmutableTag{LowerOwnerName: "o", LowerRepoName: "r", TagName: "v1.0"})
	assert.Error(t, err)

	// another path may claim the same tag name
	_, err = x.Insert(&ImmutableTag{LowerOwnerName: "o", LowerRepoName: "r2", TagName: "v1.0"})
	assert.NoError(t, err)
}
