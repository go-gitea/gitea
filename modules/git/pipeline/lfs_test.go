// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pipeline

import (
	"testing"
	"time"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindLFSFile_Batch(t *testing.T) {
	defer test.MockVariableValue(&git.DefaultFeatures().SupportCatFileBatchCommand, false)()

	testFindLFSFile(t)
}

func TestFindLFSFile_BatchCommand(t *testing.T) {
	defer test.MockVariableValue(&git.DefaultFeatures().SupportCatFileBatchCommand, true)()

	testFindLFSFile(t)
}

func testFindLFSFile(t *testing.T) {
	repoPath := "../../../tests/gitea-repositories-meta/user2/lfs.git"
	gitRepo, err := git.OpenRepository(t.Context(), repoPath)
	require.NoError(t, err)
	defer gitRepo.Close()

	objectID := git.MustIDFromString("2b6c6c4eaefa24b22f2092c3d54b263ff26feb58")

	stats, err := FindLFSFile(gitRepo, objectID)
	require.NoError(t, err)

	tm, err := time.Parse(time.RFC3339, "2022-12-21T17:56:42-05:00")
	require.NoError(t, err)

	assert.Equal(t, []*LFSResult{
		{
			Name:           "CONTRIBUTING.md",
			SHA:            "73cf03db6ece34e12bf91e8853dc58f678f2f82d",
			Summary:        "Initial commit",
			When:           tm,
			BranchName:     "master",
			FullCommitName: "master",
		},
	}, stats)
}
