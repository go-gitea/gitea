// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package tests

import (
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/modules/git/service"
	"github.com/stretchr/testify/assert"
)

func TestCommitsCount(t *testing.T) {
	RunTestPerProvider(t, func(gitService service.GitService, t *testing.T) {
		bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")

		repo, err := gitService.OpenRepository(bareRepo1Path)
		assert.NoError(t, err)
		defer repo.Close()

		commit, err := repo.GetCommit("8006ff9adbf0cb94da7dad9e537e53817f9fa5c0")
		assert.NoError(t, err)

		commitsCount, err := commit.CommitsCount()
		assert.NoError(t, err)
		assert.Equal(t, int64(3), commitsCount)
	})
}

func TestGetFullCommitID(t *testing.T) {
	RunTestPerProvider(t, func(gitService service.GitService, t *testing.T) {
		bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")

		id, err := gitService.GetFullCommitID(bareRepo1Path, "8006ff9a")
		assert.NoError(t, err)
		assert.Equal(t, "8006ff9adbf0cb94da7dad9e537e53817f9fa5c0", id)
	})
}

func TestGetFullCommitIDError(t *testing.T) {
	RunTestPerProvider(t, func(gitService service.GitService, t *testing.T) {
		bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")

		id, err := gitService.GetFullCommitID(bareRepo1Path, "unknown")
		assert.Empty(t, id)
		if assert.Error(t, err) {
			assert.EqualError(t, err, "object does not exist [id: unknown, rel_path: ]")
		}
	})
}
