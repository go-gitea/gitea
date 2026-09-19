// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"path/filepath"
	"testing"

	"gitea.dev/modules/git/gitrepo"

	"github.com/stretchr/testify/assert"
)

func TestGetNote(t *testing.T) {
	repo, err := OpenRepositoryLocal(t.Context(), filepath.Join(testReposDir, "repo1_bare"))
	assert.NoError(t, err)
	defer repo.Close()

	note, err := GetNote(t.Context(), repo, "95bb4d39648ee7e325106df01a621c530863a653")
	assert.NoError(t, err)
	assert.Equal(t, "Note contents\n", note.BlobMessage.MessageUTF8())
	assert.EqualValues(t, len(note.BlobMessage.MessageRaw), note.BlobSize)

	_, err = GetNote(t.Context(), repo, "non_existent_sha")
	assert.ErrorAs(t, err, &ErrNotExist{})
}

func TestGetNoteNestedWithCache(t *testing.T) {
	repoPath, _ := filepath.Abs(filepath.Join(testReposDir, "repo3_notes"))
	repo, err := OpenRepository(t.Context(), gitrepo.RepositoryManaged("repo3_notes", repoPath))
	assert.NoError(t, err)
	defer repo.Close()

	note, lastCommit, err := GetNoteWithLastCommit(t.Context(), repo, "ba0a96fa63532d6c5087ecef070b0250ed72fa47")
	assert.NoError(t, err)
	assert.Equal(t, "Note 1", note.BlobMessage.MessageUTF8())
	assert.Equal(t, "ba0a96fa63532d6c5087ecef070b0250ed72fa47", note.TreePath)
	assert.Equal(t, "Filip Navara", lastCommit.Author.Name)

	note, lastCommit, err = GetNoteWithLastCommit(t.Context(), repo, "3e668dbfac39cbc80a9ff9c61eb565d944453ba4")
	assert.NoError(t, err)
	assert.Equal(t, "Note 2", note.BlobMessage.MessageUTF8())
	assert.Equal(t, "3e/66/8dbfac39cbc80a9ff9c61eb565d944453ba4", note.TreePath)
	assert.Equal(t, "Filip Navara", lastCommit.Author.Name)
}
