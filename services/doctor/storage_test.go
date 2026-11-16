// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package doctor

import (
	"testing"

	git_model "gitea.dev/models/git"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/log"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateRepoLFSSizes(t *testing.T) {
	unittest.PrepareTestEnv(t)

	// repo 1 has no LFS meta objects but a stale size, repo 54 has LFS meta objects but no size
	require.NoError(t, repo_model.UpdateRepoSize(t.Context(), 1, 0, 777))
	require.NoError(t, repo_model.UpdateRepoSize(t.Context(), 54, 0, 0))
	expectedSize, err := git_model.GetRepoLFSSize(t.Context(), 54)
	require.NoError(t, err)
	require.NotZero(t, expectedSize)

	require.NoError(t, updateRepoLFSSizes(t.Context(), log.GetLogger(log.DEFAULT)))

	assert.Zero(t, unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}).LFSSize)
	assert.Equal(t, expectedSize, unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 54}).LFSSize)
}
