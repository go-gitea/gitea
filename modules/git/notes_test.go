// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetNotes(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
	bareRepo1, err := OpenRepository(bareRepo1Path)
	assert.NoError(t, err)
	defer bareRepo1.Close()

	note := Note{}
	err = GetNote(bareRepo1, "95bb4d39648ee7e325106df01a621c530863a653", &note)
	assert.NoError(t, err)
	assert.Equal(t, []byte("Note contents\n"), note.Message)
	assert.Equal(t, "Vladimir Panteleev", note.Commit.Author.Name)
}

func TestGetNestedNotes(t *testing.T) {
	repoPath := filepath.Join(testReposDir, "repo3_notes")
	repo, err := OpenRepository(repoPath)
	assert.NoError(t, err)
	defer repo.Close()

	note := Note{}
	err = GetNote(repo, "3e668dbfac39cbc80a9ff9c61eb565d944453ba4", &note)
	assert.NoError(t, err)
	assert.Equal(t, []byte("Note 2"), note.Message)
	err = GetNote(repo, "ba0a96fa63532d6c5087ecef070b0250ed72fa47", &note)
	assert.NoError(t, err)
	assert.Equal(t, []byte("Note 1"), note.Message)
}
