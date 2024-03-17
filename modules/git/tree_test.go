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
