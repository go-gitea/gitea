// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"path/filepath"
	"testing"

	"gitea.dev/modules/git/gitrepo"

	"github.com/stretchr/testify/assert"
)

func TestGetNotes(t *testing.T) {
	bareRepo1Path, _ := filepath.Abs(filepath.Join(testReposDir, "repo1_bare"))
	bareRepo1, err := OpenRepository(t.Context(), gitrepo.RepositoryManaged("repo1_bare", bareRepo1Path))
	assert.NoError(t, err)
	defer bareRepo1.Close()

	note, lastCommit, err := GetNoteWithLastCommit(t.Context(), bareRepo1, "95bb4d39648ee7e325106df01a621c530863a653")
	assert.NoError(t, err)
	assert.Equal(t, "Note contents\n", note.BlobMessage.MessageUTF8())
	assert.Equal(t, "Vladimir Panteleev", lastCommit.Author.Name)
}

func TestGetNestedNotes(t *testing.T) {
	repoPath := filepath.Join(testReposDir, "repo3_notes")
	repo, err := OpenRepositoryLocal(t.Context(), repoPath)
	assert.NoError(t, err)
	defer repo.Close()

	note, err := GetNote(t.Context(), repo, "3e668dbfac39cbc80a9ff9c61eb565d944453ba4")
	assert.NoError(t, err)
	assert.Equal(t, "Note 2", note.BlobMessage.MessageUTF8())
	note, err = GetNote(t.Context(), repo, "ba0a96fa63532d6c5087ecef070b0250ed72fa47")
	assert.NoError(t, err)
	assert.Equal(t, "Note 1", note.BlobMessage.MessageUTF8())
}

func TestGetNonExistentNotes(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
	bareRepo1, err := OpenRepositoryLocal(t.Context(), bareRepo1Path)
	assert.NoError(t, err)
	defer bareRepo1.Close()

	_, err = GetNote(t.Context(), bareRepo1, "non_existent_sha")
	assert.Error(t, err)
	assert.ErrorAs(t, err, &ErrNotExist{})
}
