// Copyright 2025 okTurtles Foundation. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestGetPublicRepositoryBySubject(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	// Create a test subject
	subject, err := repo_model.GetOrCreateSubject(ctx, "Test Subject")
	assert.NoError(t, err)
	assert.NotNil(t, subject)

	// Get a repository and assign it the subject
	repo, err := repo_model.GetRepositoryByID(ctx, 1)
	assert.NoError(t, err)
	assert.NotNil(t, repo)

	repo.SubjectID = subject.ID
	err = repo_model.UpdateRepositoryColsNoAutoTime(ctx, repo, "subject_id")
	assert.NoError(t, err)

	// Test GetPublicRepositoryBySubject
	foundRepo, err := repo_model.GetPublicRepositoryBySubject(ctx, "Test Subject")
	assert.NoError(t, err)
	assert.NotNil(t, foundRepo)
	assert.Equal(t, repo.ID, foundRepo.ID)
	assert.NotNil(t, foundRepo.SubjectRelation)
	assert.Equal(t, subject.ID, foundRepo.SubjectRelation.ID)
	assert.Equal(t, "Test Subject", foundRepo.SubjectRelation.Name)
}

func TestGetPublicRepositoryBySubject_NotFound(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	// Try to get a repository with a non-existent subject
	_, err := repo_model.GetPublicRepositoryBySubject(ctx, "Non-Existent Subject")
	assert.Error(t, err)
	assert.True(t, repo_model.IsErrSubjectNotExist(err))
}

func TestGetPublicRepositoryBySubject_PrefersRootRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	// Create a test subject
	subject, err := repo_model.GetOrCreateSubject(ctx, "Shared Subject")
	assert.NoError(t, err)

	// Get two repositories - one root and one fork
	rootRepo, err := repo_model.GetRepositoryByID(ctx, 1)
	assert.NoError(t, err)
	rootRepo.IsFork = false
	rootRepo.SubjectID = subject.ID
	err = repo_model.UpdateRepositoryColsNoAutoTime(ctx, rootRepo, "subject_id", "is_fork")
	assert.NoError(t, err)

	forkRepo, err := repo_model.GetRepositoryByID(ctx, 2)
	assert.NoError(t, err)
	forkRepo.IsFork = true
	forkRepo.ForkID = rootRepo.ID
	forkRepo.SubjectID = subject.ID
	err = repo_model.UpdateRepositoryColsNoAutoTime(ctx, forkRepo, "subject_id", "is_fork", "fork_id")
	assert.NoError(t, err)

	// GetPublicRepositoryBySubject should return the root repo, not the fork
	foundRepo, err := repo_model.GetPublicRepositoryBySubject(ctx, "Shared Subject")
	assert.NoError(t, err)
	assert.NotNil(t, foundRepo)
	assert.Equal(t, rootRepo.ID, foundRepo.ID)
	assert.False(t, foundRepo.IsFork)
}

func TestGetRepositoryByOwnerAndSubject(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	// Create a test subject
	subject, err := repo_model.GetOrCreateSubject(ctx, "Owner Subject Test")
	assert.NoError(t, err)

	// Get a repository and assign it the subject
	repo, err := repo_model.GetRepositoryByID(ctx, 1)
	assert.NoError(t, err)
	err = repo.LoadOwner(ctx)
	assert.NoError(t, err)

	repo.SubjectID = subject.ID
	err = repo_model.UpdateRepositoryColsNoAutoTime(ctx, repo, "subject_id")
	assert.NoError(t, err)

	// Test GetRepositoryByOwnerAndSubject
	foundRepo, err := repo_model.GetRepositoryByOwnerAndSubject(ctx, repo.Owner.Name, "Owner Subject Test")
	assert.NoError(t, err)
	assert.NotNil(t, foundRepo)
	assert.Equal(t, repo.ID, foundRepo.ID)
	assert.NotNil(t, foundRepo.SubjectRelation)
	assert.Equal(t, subject.ID, foundRepo.SubjectRelation.ID)
}

func TestGetRepositoryByOwnerAndSubject_NotFound(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	// Try to get a repository with a non-existent subject
	_, err := repo_model.GetRepositoryByOwnerAndSubject(ctx, "user1", "Non-Existent Subject")
	assert.Error(t, err)
	assert.True(t, repo_model.IsErrSubjectNotExist(err))
}

func TestGetRepositoryByOwnerAndSubject_WrongOwner(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	// Create a test subject
	subject, err := repo_model.GetOrCreateSubject(ctx, "Wrong Owner Test")
	assert.NoError(t, err)

	// Get a repository and assign it the subject
	repo, err := repo_model.GetRepositoryByID(ctx, 1)
	assert.NoError(t, err)
	err = repo.LoadOwner(ctx)
	assert.NoError(t, err)

	repo.SubjectID = subject.ID
	err = repo_model.UpdateRepositoryColsNoAutoTime(ctx, repo, "subject_id")
	assert.NoError(t, err)

	// Try to get the repository with a different owner
	_, err = repo_model.GetRepositoryByOwnerAndSubject(ctx, "different-user", "Wrong Owner Test")
	assert.Error(t, err)
	assert.True(t, repo_model.IsErrRepoNotExist(err))
}

func TestGetRepositoryByOwnerAndSubject_ReturnsCorrectRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()

	// Create a test subject
	subject, err := repo_model.GetOrCreateSubject(ctx, "Multi Owner Test")
	assert.NoError(t, err)

	// Get a repository and assign it the subject
	repo1, err := repo_model.GetRepositoryByID(ctx, 1)
	assert.NoError(t, err)
	err = repo1.LoadOwner(ctx)
	assert.NoError(t, err)
	repo1.SubjectID = subject.ID
	err = repo_model.UpdateRepositoryColsNoAutoTime(ctx, repo1, "subject_id")
	assert.NoError(t, err)

	// GetRepositoryByOwnerAndSubject should return the correct repository for the owner
	foundRepo, err := repo_model.GetRepositoryByOwnerAndSubject(ctx, repo1.Owner.Name, "Multi Owner Test")
	assert.NoError(t, err)
	assert.NotNil(t, foundRepo)
	assert.Equal(t, repo1.ID, foundRepo.ID)
	assert.Equal(t, repo1.Owner.Name, foundRepo.OwnerName)
}

