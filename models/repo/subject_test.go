// Copyright 2025 okTurtles Foundation. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestGetOrCreateSubject(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Test creating a new subject
	subject1, err := repo_model.GetOrCreateSubject(t.Context(), "Test Subject 1")
	assert.NoError(t, err)
	assert.NotNil(t, subject1)
	assert.Equal(t, "Test Subject 1", subject1.Name)
	assert.NotZero(t, subject1.ID)

	// Test getting existing subject
	subject2, err := repo_model.GetOrCreateSubject(t.Context(), "Test Subject 1")
	assert.NoError(t, err)
	assert.NotNil(t, subject2)
	assert.Equal(t, subject1.ID, subject2.ID)
	assert.Equal(t, subject1.Name, subject2.Name)

	// Test with empty name
	subject3, err := repo_model.GetOrCreateSubject(t.Context(), "")
	assert.NoError(t, err)
	assert.Nil(t, subject3)
}

func TestGetSubjectByID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Create a subject first
	subject1, err := repo_model.GetOrCreateSubject(t.Context(), "Test Subject 2")
	assert.NoError(t, err)
	assert.NotNil(t, subject1)

	// Get by ID
	subject2, err := repo_model.GetSubjectByID(t.Context(), subject1.ID)
	assert.NoError(t, err)
	assert.NotNil(t, subject2)
	assert.Equal(t, subject1.ID, subject2.ID)
	assert.Equal(t, subject1.Name, subject2.Name)

	// Test with non-existent ID
	_, err = repo_model.GetSubjectByID(t.Context(), 999999)
	assert.Error(t, err)
	assert.True(t, repo_model.IsErrSubjectNotExist(err))
}

func TestGetSubjectByName(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Create a subject first
	subject1, err := repo_model.GetOrCreateSubject(t.Context(), "Test Subject 3")
	assert.NoError(t, err)
	assert.NotNil(t, subject1)

	// Get by name
	subject2, err := repo_model.GetSubjectByName(t.Context(), "Test Subject 3")
	assert.NoError(t, err)
	assert.NotNil(t, subject2)
	assert.Equal(t, subject1.ID, subject2.ID)
	assert.Equal(t, subject1.Name, subject2.Name)

	// Test with non-existent name
	_, err = repo_model.GetSubjectByName(t.Context(), "Non-existent Subject")
	assert.Error(t, err)
	assert.True(t, repo_model.IsErrSubjectNotExist(err))
}

func TestRepositoryLoadSubject(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Create a subject
	subject, err := repo_model.GetOrCreateSubject(t.Context(), "Test Subject 4")
	assert.NoError(t, err)
	assert.NotNil(t, subject)

	// Get a repository (assuming repo with ID 1 exists in test data)
	repo, err := repo_model.GetRepositoryByID(t.Context(), 1)
	assert.NoError(t, err)
	assert.NotNil(t, repo)

	// Set the subject ID
	repo.SubjectID = subject.ID
	err = repo_model.UpdateRepositoryColsWithAutoTime(t.Context(), repo, "subject_id")
	assert.NoError(t, err)

	// Load the subject
	err = repo.LoadSubject(t.Context())
	assert.NoError(t, err)
	assert.NotNil(t, repo.SubjectRelation)
	assert.Equal(t, subject.ID, repo.SubjectRelation.ID)
	assert.Equal(t, subject.Name, repo.SubjectRelation.Name)

	// Test GetSubject method
	assert.Equal(t, subject.Name, repo.GetSubject())
}

func TestRepositoryGetSubjectFallback(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Get a repository
	repo, err := repo_model.GetRepositoryByID(t.Context(), 1)
	assert.NoError(t, err)
	assert.NotNil(t, repo)

	// Clear subject fields
	repo.SubjectID = 0
	repo.SubjectRelation = nil

	// Should fall back to repository name
	assert.Equal(t, repo.Name, repo.GetSubject())
}

func TestDeleteSubject(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Create a subject
	subject, err := repo_model.GetOrCreateSubject(t.Context(), "Test Subject to Delete")
	assert.NoError(t, err)
	assert.NotNil(t, subject)

	// Should be able to delete when no repos reference it
	err = repo_model.DeleteSubject(t.Context(), subject.ID)
	assert.NoError(t, err)

	// Verify it's deleted
	_, err = repo_model.GetSubjectByID(t.Context(), subject.ID)
	assert.Error(t, err)
	assert.True(t, repo_model.IsErrSubjectNotExist(err))
}

func TestDeleteSubjectInUse(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Create a subject
	subject, err := repo_model.GetOrCreateSubject(t.Context(), "Test Subject In Use")
	assert.NoError(t, err)
	assert.NotNil(t, subject)

	// Get a repository and assign the subject
	repo, err := repo_model.GetRepositoryByID(t.Context(), 1)
	assert.NoError(t, err)
	assert.NotNil(t, repo)

	repo.SubjectID = subject.ID
	err = repo_model.UpdateRepositoryColsWithAutoTime(t.Context(), repo, "subject_id")
	assert.NoError(t, err)

	// Should not be able to delete when repos reference it
	err = repo_model.DeleteSubject(t.Context(), subject.ID)
	assert.Error(t, err)
	assert.True(t, repo_model.IsErrSubjectInUse(err))
}

