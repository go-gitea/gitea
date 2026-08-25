// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestGetUserFork(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// User13 has repo 11 forked from repo10
	repo, err := repo_model.GetRepositoryByID(t.Context(), 10)
	assert.NoError(t, err)
	assert.NotNil(t, repo)
	repo, err = repo_model.GetUserFork(t.Context(), repo.ID, 13)
	assert.NoError(t, err)
	assert.NotNil(t, repo)

	repo, err = repo_model.GetRepositoryByID(t.Context(), 9)
	assert.NoError(t, err)
	assert.NotNil(t, repo)
	repo, err = repo_model.GetUserFork(t.Context(), repo.ID, 13)
	assert.NoError(t, err)
	assert.Nil(t, repo)
}

func TestGetUserForkByName(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// User13 has repo 11 ("repo11") forked from repo10
	repo, err := repo_model.GetRepositoryByID(t.Context(), 10)
	assert.NoError(t, err)
	assert.NotNil(t, repo)

	fork, err := repo_model.GetUserForkByName(t.Context(), repo.ID, 13, "repo11")
	assert.NoError(t, err)
	assert.NotNil(t, fork)

	fork, err = repo_model.GetUserForkByName(t.Context(), repo.ID, 13, "does-not-exist")
	assert.NoError(t, err)
	assert.Nil(t, fork)
}
