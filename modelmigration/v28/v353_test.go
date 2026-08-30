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
		ID            int64  `xorm:"pk autoincr"`
		RepoID        int64  `xorm:"UNIQUE(r) NOT NULL"`
		OwnerID       int64  `xorm:"INDEX(s) NOT NULL"`
		LowerRepoName string `xorm:"INDEX(s) NOT NULL"`
		LowerTagName  string `xorm:"INDEX(s) UNIQUE(r) NOT NULL"`
	}

	x, deferable := migrationtest.PrepareTestEnv(t, 0)
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	require.NoError(t, AddImmutableReleases(t.Context(), x))

	_, err := x.Insert(&ImmutableTag{RepoID: 1, OwnerID: 1, LowerRepoName: "r", LowerTagName: "v1.0"})
	require.NoError(t, err)

	// the unique index must be created by the migration, not only on fresh installs
	_, err = x.Insert(&ImmutableTag{RepoID: 1, OwnerID: 1, LowerRepoName: "r", LowerTagName: "v1.0"})
	assert.Error(t, err)

	// two repositories may claim the same name, only the path lookup has to stay non-unique
	_, err = x.Insert(&ImmutableTag{RepoID: 2, OwnerID: 1, LowerRepoName: "r", LowerTagName: "v1.0"})
	assert.NoError(t, err)
}
