// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubTree_Issue29101(t *testing.T) {
	repo, err := openRepositoryWithDefaultContext(filepath.Join(testReposDir, "repo1_bare"))
	assert.NoError(t, err)
	defer repo.Close()

	commit, err := repo.GetCommit("ce064814f4a0d337b333e646ece456cd39fab612")
	assert.NoError(t, err)

	// old code could produce a different error if called multiple times
	for i := 0; i < 10; i++ {
		_, err = commit.SubTree("file1.txt")
		assert.Error(t, err)
		assert.True(t, IsErrNotExist(err))
	}
}

func Test_GetTreePathLatestCommit(t *testing.T) {
	repo, err := openRepositoryWithDefaultContext(filepath.Join(testReposDir, "repo6_blame"))
	assert.NoError(t, err)
	defer repo.Close()

	commitID, err := repo.GetBranchCommitID("master")
	assert.NoError(t, err)
	assert.EqualValues(t, "544d8f7a3b15927cddf2299b4b562d6ebd71b6a7", commitID)

	commit, err := repo.GetTreePathLatestCommit("master", "blame.txt")
	assert.NoError(t, err)
	assert.NotNil(t, commit)
	assert.EqualValues(t, "45fb6cbc12f970b04eacd5cd4165edd11c8d7376", commit.ID.String())
}
