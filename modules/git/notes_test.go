// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetNotes(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
	bareRepo1, err := openRepositoryWithDefaultContext(bareRepo1Path)
	assert.NoError(t, err)
	defer bareRepo1.Close()

	note := Note{}
	err = GetNote(t.Context(), bareRepo1, "95bb4d39648ee7e325106df01a621c530863a653", &note)
	assert.NoError(t, err)
	assert.Equal(t, []byte("Note contents\n"), note.Message)
	assert.Equal(t, "Vladimir Panteleev", note.Commit.Author.Name)
}

func TestGetNestedNotes(t *testing.T) {
	repoPath := filepath.Join(testReposDir, "repo3_notes")
	repo, err := openRepositoryWithDefaultContext(repoPath)
	assert.NoError(t, err)
	defer repo.Close()

	note := Note{}
	err = GetNote(t.Context(), repo, "3e668dbfac39cbc80a9ff9c61eb565d944453ba4", &note)
	assert.NoError(t, err)
	assert.Equal(t, []byte("Note 2"), note.Message)
	err = GetNote(t.Context(), repo, "ba0a96fa63532d6c5087ecef070b0250ed72fa47", &note)
	assert.NoError(t, err)
	assert.Equal(t, []byte("Note 1"), note.Message)
}

func TestGetNonExistentNotes(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
	bareRepo1, err := openRepositoryWithDefaultContext(bareRepo1Path)
	assert.NoError(t, err)
	defer bareRepo1.Close()

	note := Note{}
	err = GetNote(t.Context(), bareRepo1, "non_existent_sha", &note)
	assert.Error(t, err)
	assert.IsType(t, ErrNotExist{}, err)
}
