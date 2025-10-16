// Copyright 2025 okTurtles Foundation. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"sync"
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

// TestGenerateSlugFromName tests the slug generation function with various inputs
func TestGenerateSlugFromName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple lowercase",
			input:    "the moon",
			expected: "the-moon",
		},
		{
			name:     "Capitalized",
			input:    "The Moon",
			expected: "the-moon",
		},
		{
			name:     "With exclamation",
			input:    "the moon!",
			expected: "the-moon",
		},
		{
			name:     "With question mark",
			input:    "El Camiño?",
			expected: "el-camino",
		},
		{
			name:     "With accents",
			input:    "Café Français",
			expected: "cafe-francais",
		},
		{
			name:     "With special characters",
			input:    "Hello@World#2024!",
			expected: "helloworld2024",
		},
		{
			name:     "With underscores",
			input:    "hello_world_test",
			expected: "hello-world-test",
		},
		{
			name:     "Multiple spaces",
			input:    "hello   world",
			expected: "hello-world",
		},
		{
			name:     "Leading/trailing spaces",
			input:    "  hello world  ",
			expected: "hello-world",
		},
		{
			name:     "Unicode characters",
			input:    "Zürich",
			expected: "zurich",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "subject",
		},
		{
			name:     "Only special characters",
			input:    "!!!???",
			expected: "subject",
		},
		{
			name:     "Mixed case with numbers",
			input:    "Test123Subject",
			expected: "test123subject",
		},
		{
			name:     "Multiple hyphens",
			input:    "hello---world",
			expected: "hello-world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := repo_model.GenerateSlugFromName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestCreateSubject_UniqueSlug tests that CreateSubject enforces unique slugs
func TestCreateSubject_UniqueSlug(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Create first subject
	subject1, err := repo_model.CreateSubject(t.Context(), "The Moon")
	assert.NoError(t, err)
	assert.NotNil(t, subject1)
	assert.Equal(t, "The Moon", subject1.Name)
	assert.Equal(t, "the-moon", subject1.Slug)

	// Try to create another subject with same slug (different display name)
	_, err = repo_model.CreateSubject(t.Context(), "the moon!")
	assert.Error(t, err)
	assert.True(t, repo_model.IsErrSubjectSlugAlreadyExists(err))

	// Create subject with different slug should work
	subject2, err := repo_model.CreateSubject(t.Context(), "The Sun")
	assert.NoError(t, err)
	assert.NotNil(t, subject2)
	assert.Equal(t, "The Sun", subject2.Name)
	assert.Equal(t, "the-sun", subject2.Slug)
}

// TestGetOrCreateSubject_Slug tests that GetOrCreateSubject works with slug-based uniqueness
func TestGetOrCreateSubject_Slug(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Create first subject
	subject1, err := repo_model.GetOrCreateSubject(t.Context(), "The Moon")
	assert.NoError(t, err)
	assert.NotNil(t, subject1)
	assert.Equal(t, "The Moon", subject1.Name)
	assert.Equal(t, "the-moon", subject1.Slug)

	// Get same subject with different display name but same slug
	subject2, err := repo_model.GetOrCreateSubject(t.Context(), "the moon!")
	assert.NoError(t, err)
	assert.NotNil(t, subject2)
	assert.Equal(t, subject1.ID, subject2.ID, "Should return the same subject")
	assert.Equal(t, "The Moon", subject2.Name, "Should keep original name")
	assert.Equal(t, "the-moon", subject2.Slug)

	// Create different subject
	subject3, err := repo_model.GetOrCreateSubject(t.Context(), "The Sun")
	assert.NoError(t, err)
	assert.NotNil(t, subject3)
	assert.NotEqual(t, subject1.ID, subject3.ID, "Should be different subject")
	assert.Equal(t, "The Sun", subject3.Name)
	assert.Equal(t, "the-sun", subject3.Slug)
}

// TestGetSubjectBySlug tests getting a subject by its slug
func TestGetSubjectBySlug(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Create a subject
	subject1, err := repo_model.GetOrCreateSubject(t.Context(), "Test Subject Slug")
	assert.NoError(t, err)
	assert.NotNil(t, subject1)

	// Get by slug
	subject2, err := repo_model.GetSubjectBySlug(t.Context(), "test-subject-slug")
	assert.NoError(t, err)
	assert.NotNil(t, subject2)
	assert.Equal(t, subject1.ID, subject2.ID)
	assert.Equal(t, subject1.Name, subject2.Name)
	assert.Equal(t, subject1.Slug, subject2.Slug)

	// Test with non-existent slug
	_, err = repo_model.GetSubjectBySlug(t.Context(), "non-existent-slug")
	assert.Error(t, err)
	assert.True(t, repo_model.IsErrSubjectNotExist(err))
}


// TestCreateSubject_RaceCondition tests concurrent subject creation
func TestCreateSubject_RaceCondition(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	const numGoroutines = 10
	var wg sync.WaitGroup
	errors := make([]error, numGoroutines)
	subjects := make([]*repo_model.Subject, numGoroutines)

	// Try to create the same subject concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			subject, err := repo_model.CreateSubject(t.Context(), "Concurrent Test Subject")
			errors[index] = err
			subjects[index] = subject
		}(i)
	}

	wg.Wait()

	// Count successes and failures
	successCount := 0
	failureCount := 0
	var successfulSubject *repo_model.Subject

	for i := 0; i < numGoroutines; i++ {
		if errors[i] == nil {
			successCount++
			successfulSubject = subjects[i]
		} else {
			failureCount++
			assert.True(t, repo_model.IsErrSubjectSlugAlreadyExists(errors[i]),
				"Error should be ErrSubjectSlugAlreadyExists, got: %v", errors[i])
		}
	}

	// Exactly one should succeed
	assert.Equal(t, 1, successCount, "Exactly one goroutine should succeed")
	assert.Equal(t, numGoroutines-1, failureCount, "All other goroutines should fail")
	assert.NotNil(t, successfulSubject)
	assert.Equal(t, "concurrent-test-subject", successfulSubject.Slug)
}

// TestGetOrCreateSubject_RaceCondition tests concurrent GetOrCreate operations
func TestGetOrCreateSubject_RaceCondition(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	const numGoroutines = 10
	var wg sync.WaitGroup
	subjects := make([]*repo_model.Subject, numGoroutines)
	errors := make([]error, numGoroutines)

	// Try to get or create the same subject concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			subject, err := repo_model.GetOrCreateSubject(t.Context(), "Concurrent GetOrCreate Test")
			subjects[index] = subject
			errors[index] = err
		}(i)
	}

	wg.Wait()

	// All should succeed
	for i := 0; i < numGoroutines; i++ {
		assert.NoError(t, errors[i], "GetOrCreateSubject should not fail")
		assert.NotNil(t, subjects[i])
	}

	// All should return the same subject ID
	firstID := subjects[0].ID
	for i := 1; i < numGoroutines; i++ {
		assert.Equal(t, firstID, subjects[i].ID, "All goroutines should get the same subject")
		assert.Equal(t, "concurrent-getorcreate-test", subjects[i].Slug)
	}
}

// TestMultipleRepositoriesSameSubject tests that multiple repositories can reference the same subject
func TestMultipleRepositoriesSameSubject(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Create a subject
	subject, err := repo_model.GetOrCreateSubject(t.Context(), "Shared Subject")
	assert.NoError(t, err)
	assert.NotNil(t, subject)

	// Get two different repositories
	repo1, err := repo_model.GetRepositoryByID(t.Context(), 1)
	assert.NoError(t, err)
	assert.NotNil(t, repo1)

	repo2, err := repo_model.GetRepositoryByID(t.Context(), 2)
	assert.NoError(t, err)
	assert.NotNil(t, repo2)

	// Assign the same subject to both repositories
	repo1.SubjectID = subject.ID
	err = repo_model.UpdateRepositoryColsWithAutoTime(t.Context(), repo1, "subject_id")
	assert.NoError(t, err)

	repo2.SubjectID = subject.ID
	err = repo_model.UpdateRepositoryColsWithAutoTime(t.Context(), repo2, "subject_id")
	assert.NoError(t, err)

	// Verify both repositories have the same subject
	err = repo1.LoadSubject(t.Context())
	assert.NoError(t, err)
	assert.Equal(t, subject.ID, repo1.SubjectRelation.ID)

	err = repo2.LoadSubject(t.Context())
	assert.NoError(t, err)
	assert.Equal(t, subject.ID, repo2.SubjectRelation.ID)

	// Count repositories with this subject
	count, err := repo_model.CountRepositoriesBySubject(t.Context(), subject.ID)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, count, int64(2), "At least 2 repositories should have this subject")
}


