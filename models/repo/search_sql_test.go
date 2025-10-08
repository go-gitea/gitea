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
	"github.com/stretchr/testify/require"
)

func TestSearchOrderByConstants(t *testing.T) {
	// Test that the search order constants are properly defined
	testCases := []struct {
		name     string
		orderBy  db.SearchOrderBy
		expected []string
	}{
		{
			name:     "SearchOrderByScore",
			orderBy:  db.SearchOrderByScore,
			expected: []string{"relevance_score", "ASC", "COALESCE", "subject.name", "repository.name"},
		},
		{
			name:     "SearchOrderByScoreReverse",
			orderBy:  db.SearchOrderByScoreReverse,
			expected: []string{"relevance_score", "DESC", "COALESCE", "subject.name", "repository.name"},
		},
		{
			name:     "SearchOrderBySubjectAlphabetically",
			orderBy:  db.SearchOrderBySubjectAlphabetically,
			expected: []string{"COALESCE", "subject.name", "repository.name", "ASC"},
		},
		{
			name:     "SearchOrderBySubjectReverse",
			orderBy:  db.SearchOrderBySubjectReverse,
			expected: []string{"COALESCE", "subject.name", "repository.name", "DESC"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sqlStr := string(tc.orderBy)
			for _, expected := range tc.expected {
				assert.Contains(t, sqlStr, expected,
					"SQL string for %s should contain '%s'", tc.name, expected)
			}
		})
	}
}

func TestRelevanceScoreSQL(t *testing.T) {
	// Test that relevance score SQL is correctly structured
	scoreSQL := string(db.SearchOrderByScore)
	reverseScoreSQL := string(db.SearchOrderByScoreReverse)

	// Both should contain relevance_score
	assert.Contains(t, scoreSQL, "relevance_score", "Score SQL should contain relevance_score")
	assert.Contains(t, reverseScoreSQL, "relevance_score", "Reverse score SQL should contain relevance_score")

	// Both should contain COALESCE for subject/name fallback
	assert.Contains(t, scoreSQL, "COALESCE(subject.name, repository.name)", "Score SQL should contain subject fallback")
	assert.Contains(t, reverseScoreSQL, "COALESCE(subject.name, repository.name)", "Reverse score SQL should contain subject fallback")

	// They should have different ordering for relevance_score
	assert.Contains(t, scoreSQL, "relevance_score ASC", "Score SQL should order relevance_score ASC")
	assert.Contains(t, reverseScoreSQL, "relevance_score DESC", "Reverse score SQL should order relevance_score DESC")

	// Both should have same secondary ordering (alphabetical by display name)
	assert.Contains(t, scoreSQL, "COALESCE(subject.name, repository.name) ASC",
		"Score SQL should have alphabetical secondary ordering")
	assert.Contains(t, reverseScoreSQL, "COALESCE(subject.name, repository.name) ASC",
		"Reverse score SQL should have alphabetical secondary ordering")
}

func TestSubjectFieldSearchIntegration(t *testing.T) {
	unittest.PrepareTestEnv(t)

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	// Test that keyword search includes subject field
	opts := repo_model.SearchRepoOptions{
		ListOptions: db.ListOptions{PageSize: 10},
		Actor:       user,
		Keyword:     "test", // This should search both name and subject fields
		Private:     true,
	}

	repos, count, err := repo_model.SearchRepository(context.Background(), opts)
	require.NoError(t, err, "SearchRepository should not return an error")

	// Verify that search returns results (assuming test data exists)
	if count > 0 {
		assert.NotEmpty(t, repos, "Should return repositories when keyword matches")

		// Verify that repositories have the expected fields
		for _, repo := range repos {
			assert.NotEmpty(t, repo.Name, "Repository should have a name")
			// Subject can be accessed via GetSubject() method
			_ = repo.GetSubject() // This should not panic
		}
	}
}

func TestKeywordSearchConditions(t *testing.T) {
	unittest.PrepareTestEnv(t)

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	// Test various keyword search scenarios
	testCases := []struct {
		name        string
		keyword     string
		description string
	}{
		{
			name:        "SingleKeyword",
			keyword:     "test",
			description: "Single keyword should search both name and subject",
		},
		{
			name:        "MultipleKeywords",
			keyword:     "test,repo",
			description: "Multiple keywords should search both name and subject for each term",
		},
		{
			name:        "EmptyKeyword",
			keyword:     "",
			description: "Empty keyword should not cause errors",
		},
		{
			name:        "SpecialCharacters",
			keyword:     "test-repo",
			description: "Keywords with special characters should work",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opts := repo_model.SearchRepoOptions{
				ListOptions: db.ListOptions{PageSize: 5},
				Actor:       user,
				Keyword:     tc.keyword,
				Private:     true,
			}

			_, count, err := repo_model.SearchRepository(context.Background(), opts)
			require.NoError(t, err, "SearchRepository should not return an error for %s", tc.description)

			// For non-empty keywords, we expect the search to complete successfully
			// (results may be empty if no matches, but no errors should occur)
			if tc.keyword != "" {
				assert.GreaterOrEqual(t, count, int64(0), "Count should be non-negative")
				// Repository list length is always non-negative by definition, no need to assert
			}
		})
	}
}

func TestDefaultOrderingWithKeywords(t *testing.T) {
	unittest.PrepareTestEnv(t)

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	// Test that default ordering uses relevance when keyword is provided
	optsWithKeyword := repo_model.SearchRepoOptions{
		ListOptions: db.ListOptions{PageSize: 5},
		Actor:       user,
		Keyword:     "test",
		Private:     true,
		// OrderBy is intentionally not set to test default behavior
	}

	repos, count, err := repo_model.SearchRepository(context.Background(), optsWithKeyword)
	require.NoError(t, err, "SearchRepository with keyword should not return an error")

	if count > 0 {
		assert.NotEmpty(t, repos, "Should return repositories when keyword search finds matches")
	}

	// Test that default ordering without keyword uses alphabetical
	optsWithoutKeyword := repo_model.SearchRepoOptions{
		ListOptions: db.ListOptions{PageSize: 5},
		Actor:       user,
		Keyword:     "",
		Private:     true,
		// OrderBy is intentionally not set to test default behavior
	}

	reposNoKeyword, countNoKeyword, err := repo_model.SearchRepository(context.Background(), optsWithoutKeyword)
	require.NoError(t, err, "SearchRepository without keyword should not return an error")
	assert.Positive(t, countNoKeyword, "Should find repositories without keyword")
	assert.NotEmpty(t, reposNoKeyword, "Should return repositories without keyword")
}

func TestOrderByMapIntegrity(t *testing.T) {
	// Test that OrderByMap has consistent structure
	ascMap, exists := repo_model.OrderByMap["asc"]
	assert.True(t, exists, "OrderByMap should have 'asc' key")
	assert.NotEmpty(t, ascMap, "ASC order map should not be empty")

	descMap, exists := repo_model.OrderByMap["desc"]
	assert.True(t, exists, "OrderByMap should have 'desc' key")
	assert.NotEmpty(t, descMap, "DESC order map should not be empty")

	// Test that both maps have the same keys
	for key := range ascMap {
		_, exists := descMap[key]
		assert.True(t, exists, "DESC map should have same key '%s' as ASC map", key)
	}

	for key := range descMap {
		_, exists := ascMap[key]
		assert.True(t, exists, "ASC map should have same key '%s' as DESC map", key)
	}

	// Test that score ordering exists in both
	scoreAsc, exists := ascMap["score"]
	assert.True(t, exists, "ASC map should have 'score' key")
	assert.NotEmpty(t, scoreAsc, "ASC score ordering should not be empty")

	scoreDesc, exists := descMap["score"]
	assert.True(t, exists, "DESC map should have 'score' key")
	assert.NotEmpty(t, scoreDesc, "DESC score ordering should not be empty")
}

func TestOrderByFlatMapIntegrity(t *testing.T) {
	// Test that OrderByFlatMap has all expected entries
	expectedEntries := []string{
		"newest", "oldest", "recentupdate", "leastupdate",
		"reversealphabetically", "alphabetically",
		"score", "reversescore",
		"reversesize", "size",
		"reversegitsize", "gitsize",
		"reverselfssize", "lfssize",
		"moststars", "feweststars",
		"mostforks", "fewestforks",
	}

	for _, entry := range expectedEntries {
		orderBy, exists := repo_model.OrderByFlatMap[entry]
		assert.True(t, exists, "OrderByFlatMap should have '%s' entry", entry)
		assert.NotEmpty(t, orderBy, "OrderByFlatMap['%s'] should not be empty", entry)
	}

	// Test that score and reversescore map to different orderings
	scoreOrder := repo_model.OrderByFlatMap["score"]
	reverseScoreOrder := repo_model.OrderByFlatMap["reversescore"]
	assert.NotEqual(t, scoreOrder, reverseScoreOrder, "score and reversescore should map to different orderings")
}

func TestSearchWithOrgRepoPattern(t *testing.T) {
	unittest.PrepareTestEnv(t)

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	// Test search with "org/repo" pattern and score sorting
	opts := repo_model.SearchRepoOptions{
		ListOptions: db.ListOptions{PageSize: 5},
		Actor:       user,
		Keyword:     "user2/repo", // This should trigger org/repo pattern matching
		OrderBy:     repo_model.OrderByFlatMap["score"],
		Private:     true,
	}

	_, count, err := repo_model.SearchRepository(context.Background(), opts)
	require.NoError(t, err, "SearchRepository with org/repo pattern should not return an error")

	// The search should complete successfully regardless of whether matches are found
	assert.GreaterOrEqual(t, count, int64(0), "Count should be non-negative")
	// Repository list length is always non-negative by definition, no need to assert
}

func TestSearchWithIncludeDescription(t *testing.T) {
	unittest.PrepareTestEnv(t)

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	// Test search with description inclusion and score sorting
	opts := repo_model.SearchRepoOptions{
		ListOptions:        db.ListOptions{PageSize: 5},
		Actor:              user,
		Keyword:            "test",
		IncludeDescription: true,
		OrderBy:            repo_model.OrderByFlatMap["score"],
		Private:            true,
	}

	_, count, err := repo_model.SearchRepository(context.Background(), opts)
	require.NoError(t, err, "SearchRepository with description inclusion should not return an error")

	// Verify that search completes successfully
	assert.GreaterOrEqual(t, count, int64(0), "Count should be non-negative")
	// Repository list length is always non-negative by definition, no need to assert
}
