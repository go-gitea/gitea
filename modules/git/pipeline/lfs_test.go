// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pipeline

import (
	"testing"
	"time"

	"code.gitea.io/gitea/modules/git"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindLFSFile(t *testing.T) {
	repoPath := "../../../tests/gitea-repositories-meta/user2/lfs.git"
	gitRepo, err := git.OpenRepository(t.Context(), repoPath)
	require.NoError(t, err)
	defer gitRepo.Close()

	objectID := git.MustIDFromString("2b6c6c4eaefa24b22f2092c3d54b263ff26feb58")

	stats, err := FindLFSFile(gitRepo, objectID)
	require.NoError(t, err)

	tm, err := time.Parse(time.RFC3339, "2022-12-21T17:56:42-05:00")
	require.NoError(t, err)

	assert.Len(t, stats, 1)
	assert.Equal(t, "CONTRIBUTING.md", stats[0].Name)
	assert.Equal(t, "73cf03db6ece34e12bf91e8853dc58f678f2f82d", stats[0].SHA)
	assert.Equal(t, "Initial commit", stats[0].Summary)
	assert.Equal(t, tm, stats[0].When)
	assert.Empty(t, stats[0].ParentHashes)
	assert.Equal(t, "master", stats[0].BranchName)
	assert.Equal(t, "master", stats[0].FullCommitName)
}
