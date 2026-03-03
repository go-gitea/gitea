// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
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

func TestReparentForkWithGrandparent(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Use repo 11 (forked from 10)
	// User13 has repo 11 forked from repo10
	grandParentRepo, err := repo_model.GetRepositoryByID(t.Context(), 10)
	assert.NoError(t, err)
	initialGrandParentForks := grandParentRepo.NumForks

	forkedRepo, err := repo_model.GetRepositoryByID(t.Context(), 11)
	assert.NoError(t, err)
	assert.True(t, forkedRepo.IsFork)
	assert.Equal(t, int64(10), forkedRepo.ForkID)

	// User2 has repo 1 forked from repo 11
	// (Actually in fixtures repo 11 is user13/repo11, let's check if there's a fork of 11)
	// We'll create a new fork for the test
	newFork := &repo_model.Repository{
		OwnerID: 1,
		Name:    "fork-of-11",
		IsFork:  true,
		ForkID:  11,
	}
	assert.NoError(t, db.Insert(t.Context(), newFork))
	assert.NoError(t, repo_model.IncrementRepoForkNum(t.Context(), 11))

	// Refresh forkedRepo
	forkedRepo, _ = repo_model.GetRepositoryByID(t.Context(), 11)
	assert.Equal(t, 1, forkedRepo.NumForks)

	// Perform reparenting: 11 becomes fork of newFork
	// This means 11 is no longer a fork of 10.
	err = repo_model.ReparentFork(t.Context(), newFork.ID, 11)
	assert.NoError(t, err)

	// Verify grandParentRepo (10) has one less fork
	grandParentRepoAfter, err := repo_model.GetRepositoryByID(t.Context(), 10)
	assert.NoError(t, err)
	assert.Equal(t, initialGrandParentForks-1, grandParentRepoAfter.NumForks)

	// Verify newFork is now a parent
	newForkAfter, err := repo_model.GetRepositoryByID(t.Context(), newFork.ID)
	assert.NoError(t, err)
	assert.False(t, newForkAfter.IsFork)
	assert.Equal(t, int64(0), newForkAfter.ForkID)
	assert.Equal(t, 1, newForkAfter.NumForks)

	// Verify 11 is now a fork of newFork
	forkedRepoAfter, err := repo_model.GetRepositoryByID(t.Context(), 11)
	assert.NoError(t, err)
	assert.True(t, forkedRepoAfter.IsFork)
	assert.Equal(t, newFork.ID, forkedRepoAfter.ForkID)
}
