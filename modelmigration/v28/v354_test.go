// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"maps"
	"slices"
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/require"
	"xorm.io/xorm/schemas"
)

func TestAddRepoSHAIndexToCommitStatus(t *testing.T) {
	type CommitStatus struct { // the schema before the migration
		ID     int64  `xorm:"pk autoincr"`
		Index  int64  `xorm:"INDEX UNIQUE(repo_sha_index)"`
		RepoID int64  `xorm:"INDEX UNIQUE(repo_sha_index)"`
		SHA    string `xorm:"VARCHAR(64) NOT NULL INDEX UNIQUE(repo_sha_index)"`
	}

	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(CommitStatus))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	hasRepoSHAIndex := func() bool {
		indexes, err := x.Dialect().GetIndexes(x.DB(), t.Context(), "commit_status")
		require.NoError(t, err)
		return slices.ContainsFunc(slices.Collect(maps.Values(indexes)), func(idx *schemas.Index) bool {
			return slices.Equal(idx.Cols, []string{"repo_id", "sha"})
		})
	}

	require.False(t, hasRepoSHAIndex())
	require.NoError(t, AddRepoSHAIndexToCommitStatus(t.Context(), x))
	require.True(t, hasRepoSHAIndex())
	require.NoError(t, AddRepoSHAIndexToCommitStatus(t.Context(), x), "re-running must not fail on the existing index")
}
