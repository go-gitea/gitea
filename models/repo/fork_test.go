// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"

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

func TestReparentFork(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Use repo 11 (forked from 10)
	forkedRepo, err := repo_model.GetRepositoryByID(t.Context(), 11)
	assert.NoError(t, err)
	assert.True(t, forkedRepo.IsFork)
	assert.Equal(t, int64(10), forkedRepo.ForkID)

	parentRepo, err := repo_model.GetRepositoryByID(t.Context(), 10)
	assert.NoError(t, err)
	assert.False(t, parentRepo.IsFork)
	assert.Equal(t, int64(0), parentRepo.ForkID)

	// Perform reparenting: 10 becomes fork of 11
	err = repo_model.ReparentFork(t.Context(), 11, 10)
	assert.NoError(t, err)

	// Verify the swap
	forkedRepoAfter, err := repo_model.GetRepositoryByID(t.Context(), 11)
	assert.NoError(t, err)
	assert.False(t, forkedRepoAfter.IsFork)
	assert.Equal(t, int64(0), forkedRepoAfter.ForkID)
	assert.Equal(t, 1, forkedRepoAfter.NumForks)

	parentRepoAfter, err := repo_model.GetRepositoryByID(t.Context(), 10)
	assert.NoError(t, err)
	assert.True(t, parentRepoAfter.IsFork)
	assert.Equal(t, int64(11), parentRepoAfter.ForkID)
}
