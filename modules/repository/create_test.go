// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"

	repo_model "codenest.dev/models/repo"
	"codenest.dev/models/unittest"
	"codenest.dev/modules/git"

	"github.com/stretchr/testify/assert"
)

func TestGetDirectorySize(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	repo, err := repo_model.GetRepositoryByID(t.Context(), 1)
	assert.NoError(t, err)
	size, err := git.CalcRepositorySize(repo)
	assert.NoError(t, err)
	repo.Size = 8213 // real size on the disk
	assert.Equal(t, repo.Size, size)
}
