// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubTree_Issue29101(t *testing.T) {
	repo, err := OpenRepositoryLocal(t.Context(), filepath.Join(testReposDir, "repo1_bare"))
	assert.NoError(t, err)
	defer repo.Close()

	commit, err := repo.GetCommit(t.Context(), "ce064814f4a0d337b333e646ece456cd39fab612")
	assert.NoError(t, err)

	// old code could produce a different error if called multiple times
	for range 10 {
		_, err = commit.SubTree(t.Context(), repo, "file1.txt")
		assert.Error(t, err)
		assert.True(t, IsErrNotExist(err))
	}
}

func TestTreeListEntriesResolvesTagAndRejectsBlob(t *testing.T) {
	repo, err := OpenRepositoryLocal(t.Context(), filepath.Join(testReposDir, "repo1_bare"))
	require.NoError(t, err)
	defer repo.Close()

	entries, err := newTree(MustIDFromString("3ad28a9149a2864384548f3d17ed7f38014c9e8a")).ListEntries(t.Context(), repo)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	assert.Equal(t, []string{"file1.txt", "file2.txt", "foo"}, names)
	assert.True(t, entries[2].IsDir())

	_, err = newTree(MustIDFromString("e2129701f1a4d54dc44f03c93bca0a2aec7c5449")).ListEntries(t.Context(), repo)
	assert.True(t, IsErrNotExist(err))
}

func Test_GetTreePathLatestCommit(t *testing.T) {
	repo, err := OpenRepositoryLocal(t.Context(), filepath.Join(testReposDir, "repo6_blame"))
	assert.NoError(t, err)
	defer repo.Close()

	commitID, err := repo.GetBranchCommitID(t.Context(), "master")
	assert.NoError(t, err)
	assert.Equal(t, "544d8f7a3b15927cddf2299b4b562d6ebd71b6a7", commitID)

	commit, err := repo.GetTreePathLatestCommit(t.Context(), "master", "blame.txt")
	assert.NoError(t, err)
	assert.NotNil(t, commit)
	assert.Equal(t, "45fb6cbc12f970b04eacd5cd4165edd11c8d7376", commit.ID.String())
}
