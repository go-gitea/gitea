// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package tests

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/modules/git/service"
	"github.com/stretchr/testify/assert"
)

func TestGetNotes(t *testing.T) {
	RunTestPerProvider(t, func(service service.GitService, t *testing.T) {
		bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
		bareRepo1, err := service.OpenRepository(bareRepo1Path)
		assert.NoError(t, err)
		defer bareRepo1.Close()

		reader, commit, err := service.GetNote(bareRepo1, "95bb4d39648ee7e325106df01a621c530863a653")
		assert.NoError(t, err)
		if err != nil {
			return
		}
		defer reader.Close()
		d, err := ioutil.ReadAll(reader)
		assert.NoError(t, err)
		if err != nil {
			return
		}

		assert.Equal(t, []byte("Note contents\n"), d)
		assert.Equal(t, "Vladimir Panteleev", commit.Author().Name)
	})
}

func TestGetNestedNotes(t *testing.T) {
	RunTestPerProvider(t, func(service service.GitService, t *testing.T) {
		repoPath := filepath.Join(testReposDir, "repo3_notes")
		repo, err := service.OpenRepository(repoPath)
		assert.NoError(t, err)
		defer repo.Close()

		reader, _, err := service.GetNote(repo, "3e668dbfac39cbc80a9ff9c61eb565d944453ba4")
		assert.NoError(t, err)
		if err != nil {
			return
		}
		defer reader.Close()
		d, err := ioutil.ReadAll(reader)
		assert.NoError(t, err)
		if err != nil {
			return
		}
		assert.NoError(t, err)
		assert.Equal(t, []byte("Note 2"), d)
		reader, _, err = service.GetNote(repo, "ba0a96fa63532d6c5087ecef070b0250ed72fa47")
		assert.NoError(t, err)
		if err != nil {
			return
		}
		defer reader.Close()
		d, err = ioutil.ReadAll(reader)
		assert.NoError(t, err)
		if err != nil {
			return
		}

		assert.Equal(t, []byte("Note 1"), d)
	})
}
