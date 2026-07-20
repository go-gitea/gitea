// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestRepository_GitRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	assert.Equal(t, "user2/repo1.git", repo_model.CodeRepoByName(repo.OwnerName, repo.Name).GitRepoLocation())
	assert.Equal(t, "user2/repo1.git", repo.CodeStorageRepo().GitRepoLocation())
	assert.Equal(t, "repo-1", repo.CodeStorageRepo().GitRepoManagedID())

	assert.Equal(t, "user2/repo1.wiki.git", repo_model.WikiRepoByName(repo.OwnerName, repo.Name).GitRepoLocation())
	assert.Equal(t, "user2/repo1.wiki.git", repo.WikiStorageRepo().GitRepoLocation())
	assert.Equal(t, "repo-wiki-1", repo.WikiStorageRepo().GitRepoManagedID())
}
