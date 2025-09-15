// Copyright 2025 okTurtles Foundation. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"context"
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestIsRepositoryNameGloballyUnique(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	ctx := context.Background()

	// Test with a name that doesn't exist
	unique, err := repo_model.IsRepositoryNameGloballyUnique(ctx, "unique-test-repo")
	assert.NoError(t, err)
	assert.True(t, unique)

	// Test with a name that exists (repo1 exists in test data)
	unique, err = repo_model.IsRepositoryNameGloballyUnique(ctx, "repo1")
	assert.NoError(t, err)
	assert.False(t, unique)

	// Test case insensitive matching
	unique, err = repo_model.IsRepositoryNameGloballyUnique(ctx, "REPO1")
	assert.NoError(t, err)
	assert.False(t, unique)
}

func TestIsRepositorySubjectGloballyUnique(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	ctx := context.Background()

	// Test with empty subject (should be allowed)
	unique, err := repo_model.IsRepositorySubjectGloballyUnique(ctx, "")
	assert.NoError(t, err)
	assert.True(t, unique)

	// Test with a subject that doesn't exist
	unique, err = repo_model.IsRepositorySubjectGloballyUnique(ctx, "Unique Test Subject")
	assert.NoError(t, err)
	assert.True(t, unique)

	// Create a test repository with a subject to test against
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	repo := &repo_model.Repository{
		OwnerID:     user.ID,
		Owner:       user,
		LowerName:   "test-subject-repo",
		Name:        "test-subject-repo",
		Subject:     "Test Subject for Global Uniqueness",
		Description: "Test repository for global uniqueness validation",
		IsPrivate:   false,
	}

	_, err = db.GetEngine(ctx).Insert(repo)
	assert.NoError(t, err)

	// Test with the same subject (should not be unique)
	unique, err = repo_model.IsRepositorySubjectGloballyUnique(ctx, "Test Subject for Global Uniqueness")
	assert.NoError(t, err)
	assert.False(t, unique)

	// Test case insensitive matching
	unique, err = repo_model.IsRepositorySubjectGloballyUnique(ctx, "test subject for global uniqueness")
	assert.NoError(t, err)
	assert.False(t, unique)

	// Test with whitespace variations
	unique, err = repo_model.IsRepositorySubjectGloballyUnique(ctx, "  Test Subject for Global Uniqueness  ")
	assert.NoError(t, err)
	assert.False(t, unique)

	// Clean up
	_, err = db.GetEngine(ctx).Delete(repo)
	assert.NoError(t, err)
}

func TestCheckCreateRepositoryGlobalUnique(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	ctx := context.Background()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	// Test with unique name and subject
	err := repo_model.CheckCreateRepositoryGlobalUnique(ctx, user, user, "unique-global-repo", "Unique Global Subject", false)
	assert.NoError(t, err)

	// Test with existing repository name (repo1 exists)
	err = repo_model.CheckCreateRepositoryGlobalUnique(ctx, user, user, "repo1", "Some Subject", false)
	assert.True(t, repo_model.IsErrRepoNameGloballyTaken(err))

	// Create a test repository to test subject uniqueness
	testRepo := &repo_model.Repository{
		OwnerID:     user.ID,
		Owner:       user,
		LowerName:   "test-global-subject",
		Name:        "test-global-subject",
		Subject:     "Global Test Subject",
		Description: "Test repository for global subject validation",
		IsPrivate:   false,
	}

	_, err = db.GetEngine(ctx).Insert(testRepo)
	assert.NoError(t, err)

	// Test with existing subject
	err = repo_model.CheckCreateRepositoryGlobalUnique(ctx, user, user, "another-unique-repo", "Global Test Subject", false)
	assert.True(t, repo_model.IsErrRepoSubjectGloballyTaken(err))

	// Test with empty subject (should be allowed)
	err = repo_model.CheckCreateRepositoryGlobalUnique(ctx, user, user, "repo-without-subject", "", false)
	assert.NoError(t, err)

	// Clean up
	_, err = db.GetEngine(ctx).Delete(testRepo)
	assert.NoError(t, err)
}

func TestErrRepoNameGloballyTaken(t *testing.T) {
	err := repo_model.ErrRepoNameGloballyTaken{Name: "test-repo"}
	assert.True(t, repo_model.IsErrRepoNameGloballyTaken(err))
	assert.Contains(t, err.Error(), "test-repo")
	assert.Contains(t, err.Error(), "globally")
}

func TestErrRepoSubjectGloballyTaken(t *testing.T) {
	err := repo_model.ErrRepoSubjectGloballyTaken{Subject: "Test Subject"}
	assert.True(t, repo_model.IsErrRepoSubjectGloballyTaken(err))
	assert.Contains(t, err.Error(), "Test Subject")
	assert.Contains(t, err.Error(), "globally")
}
