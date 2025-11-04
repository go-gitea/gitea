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

func TestIsRepositorySubjectGloballyUnique(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	ctx := context.Background()

	// Test with empty subject (should not be allowed)
	unique, err := repo_model.IsRepositorySubjectGloballyUnique(ctx, "")
	assert.NoError(t, err)
	assert.False(t, unique)

	// Test with a subject that doesn't exist
	unique, err = repo_model.IsRepositorySubjectGloballyUnique(ctx, "Unique Test Subject")
	assert.NoError(t, err)
	assert.True(t, unique)

	// Create a test subject to test against
	subject, err := repo_model.GetOrCreateSubject(ctx, "Test Subject for Global Uniqueness")
	assert.NoError(t, err)
	assert.NotNil(t, subject)

	// Create a test repository with the subject
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	repo := &repo_model.Repository{
		OwnerID:     user.ID,
		Owner:       user,
		LowerName:   "test-subject-repo",
		Name:        "test-subject-repo",
		SubjectID:   subject.ID,
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
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	// Test with unique name and subject
	err := repo_model.CheckCreateRepositoryGlobalUnique(ctx, user, user, "unique-global-repo", "Unique Global Subject", false)
	assert.NoError(t, err)

	// Test with existing repository name owned by same user (should fail - owner-scoped uniqueness)
	// user2 owns repo1, so user2 cannot create another repo1
	err = repo_model.CheckCreateRepositoryGlobalUnique(ctx, user2, user2, "repo1", "Some Subject", false)
	assert.True(t, repo_model.IsErrRepoAlreadyExist(err), "Should fail when same owner tries to create repo with duplicate name")

	// Test with existing repository name owned by different user (should succeed - no global uniqueness)
	// user2 already has repo1, but user (user1) should be able to create their own repo1
	err = repo_model.CheckCreateRepositoryGlobalUnique(ctx, user, user, "repo1", "Different Subject", false)
	assert.NoError(t, err, "Different owners should be able to have repositories with the same name")

	// Create a test subject and repository to test subject uniqueness
	globalSubject, err := repo_model.GetOrCreateSubject(ctx, "Global Test Subject")
	assert.NoError(t, err)

	testRepo := &repo_model.Repository{
		OwnerID:     user.ID,
		Owner:       user,
		LowerName:   "test-global-subject",
		Name:        "test-global-subject",
		SubjectID:   globalSubject.ID,
		Description: "Test repository for global subject validation",
		IsPrivate:   false,
	}

	_, err = db.GetEngine(ctx).Insert(testRepo)
	assert.NoError(t, err)

	// Test with existing subject (should fail - subjects are globally unique)
	err = repo_model.CheckCreateRepositoryGlobalUnique(ctx, user, user, "another-unique-repo", "Global Test Subject", false)
	assert.True(t, repo_model.IsErrRepoSubjectGloballyTaken(err), "Should fail when subject is already taken globally")

	// Test with empty subject (should fail - empty subjects are not allowed)
	err = repo_model.CheckCreateRepositoryGlobalUnique(ctx, user, user, "repo-without-subject", "", false)
	assert.True(t, repo_model.IsErrRepoSubjectGloballyTaken(err), "Should fail when subject is empty")

	// Clean up
	_, err = db.GetEngine(ctx).Delete(testRepo)
	assert.NoError(t, err)
}

func TestErrRepoSubjectGloballyTaken(t *testing.T) {
	err := repo_model.ErrRepoSubjectGloballyTaken{Subject: "Test Subject"}
	assert.True(t, repo_model.IsErrRepoSubjectGloballyTaken(err))
	assert.Contains(t, err.Error(), "Test Subject")
	assert.Contains(t, err.Error(), "globally")
}
