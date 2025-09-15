// Copyright 2025 okTurtles Foundation. All rights reserved.
// SPDX-License-Identifier: MIT

package repo_test

import (
	"context"
	"sort"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockRepository represents a test repository for relevance testing
type MockRepository struct {
	Name    string
	Subject string
	Score   int
}

func TestRelevanceSortingOrder(t *testing.T) {
	// Test the complete relevance sorting order with realistic repository examples
	testRepos := []MockRepository{
		{Name: "moon", Subject: "Moon", Score: 0},                                // Exact match in subject
		{Name: "moon-landing", Subject: "Moon Landing", Score: 0},                // Prefix match in subject
		{Name: "dark-side-moon", Subject: "The Dark Side of The Moon", Score: 0}, // Substring match in subject
		{Name: "honeymoon-planning", Subject: "Honeymoon Planning", Score: 0},    // Fuzzy match in subject
		{Name: "moon-project", Subject: "", Score: 0},                            // Exact match in name (no subject)
		{Name: "lunar-mission", Subject: "Lunar Mission", Score: 0},              // No match
		{Name: "space-exploration", Subject: "Moon Base Alpha", Score: 0},        // Prefix match in subject
		{Name: "apollo-program", Subject: "Apollo Moon Program", Score: 0},       // Substring match in subject
	}

	keyword := "moon"

	// Calculate relevance scores
	for i := range testRepos {
		testRepos[i].Score = calculateMockRelevanceScore(testRepos[i].Name, testRepos[i].Subject, keyword)
	}

	// Test "score" sorting (best relevance first - ascending score)
	scoreRepos := make([]MockRepository, len(testRepos))
	copy(scoreRepos, testRepos)
	sortByScore(scoreRepos, true)

	t.Run("ScoreSorting", func(t *testing.T) {
		// Verify exact matches come first
		assert.Equal(t, 1, scoreRepos[0].Score, "First repository should have exact match (score 1)")
		assert.Equal(t, "Moon", scoreRepos[0].Subject, "First repository should be exact match 'Moon'")

		// Verify prefix matches come second
		prefixCount := 0
		for i := 1; i < len(scoreRepos) && scoreRepos[i].Score == 2; i++ {
			prefixCount++
		}
		assert.Positive(t, prefixCount, "Should have prefix matches after exact matches")

		// Verify substring matches come third
		substringCount := 0
		for i := range scoreRepos {
			if scoreRepos[i].Score == 3 {
				substringCount++
			}
		}
		assert.Positive(t, substringCount, "Should have substring matches")

		// Verify no matches come last
		lastRepo := scoreRepos[len(scoreRepos)-1]
		assert.Equal(t, 4, lastRepo.Score, "Last repository should have no match (score 4)")
	})

	// Test "reversescore" sorting (worst relevance first - descending score)
	reverseScoreRepos := make([]MockRepository, len(testRepos))
	copy(reverseScoreRepos, testRepos)
	sortByScore(reverseScoreRepos, false)

	t.Run("ReverseScoreSorting", func(t *testing.T) {
		// Verify no matches come first in reverse sorting
		assert.Equal(t, 4, reverseScoreRepos[0].Score, "First repository in reverse should have no match (score 4)")

		// Verify exact matches come last in reverse sorting
		lastRepo := reverseScoreRepos[len(reverseScoreRepos)-1]
		assert.Equal(t, 1, lastRepo.Score, "Last repository in reverse should have exact match (score 1)")
		assert.Equal(t, "Moon", lastRepo.Subject, "Last repository in reverse should be exact match 'Moon'")
	})
}

func TestSecondaryAlphabeticalSorting(t *testing.T) {
	// Test that repositories with the same relevance score are sorted alphabetically
	testRepos := []MockRepository{
		{Name: "zebra-moon", Subject: "Zebra Moon Project", Score: 0}, // Substring match
		{Name: "alpha-moon", Subject: "Alpha Moon Project", Score: 0}, // Substring match
		{Name: "beta-moon", Subject: "Beta Moon Project", Score: 0},   // Substring match
	}

	keyword := "moon"

	// Calculate scores (should all be substring matches = score 3)
	for i := range testRepos {
		testRepos[i].Score = calculateMockRelevanceScore(testRepos[i].Name, testRepos[i].Subject, keyword)
	}

	// Sort by score (with secondary alphabetical sorting)
	sortByScore(testRepos, true)

	// Verify all have same relevance score
	for _, repo := range testRepos {
		assert.Equal(t, 3, repo.Score, "All repositories should have substring match score")
	}

	// Verify alphabetical order within same score
	assert.Equal(t, "Alpha Moon Project", testRepos[0].Subject, "First should be Alpha (alphabetically)")
	assert.Equal(t, "Beta Moon Project", testRepos[1].Subject, "Second should be Beta (alphabetically)")
	assert.Equal(t, "Zebra Moon Project", testRepos[2].Subject, "Third should be Zebra (alphabetically)")
}

func TestEmptySubjectFallback(t *testing.T) {
	// Test that repositories without subjects fall back to using name for relevance
	testRepos := []MockRepository{
		{Name: "moon-project", Subject: "", Score: 0},           // Should use name for matching
		{Name: "project-moon", Subject: "", Score: 0},           // Should use name for matching
		{Name: "lunar-project", Subject: "", Score: 0},          // Should use name for matching
		{Name: "space-project", Subject: "Moon Base", Score: 0}, // Should use subject for matching
	}

	keyword := "moon"

	// Calculate scores
	for i := range testRepos {
		testRepos[i].Score = calculateMockRelevanceScore(testRepos[i].Name, testRepos[i].Subject, keyword)
	}

	// Verify scoring behavior
	assert.Equal(t, 2, testRepos[0].Score, "moon-project should be prefix match in name")
	assert.Equal(t, 3, testRepos[1].Score, "project-moon should be substring match in name")
	assert.Equal(t, 4, testRepos[2].Score, "lunar-project should be no match")
	assert.Equal(t, 2, testRepos[3].Score, "space-project should be prefix match in subject")
}

func TestCaseInsensitiveMatching(t *testing.T) {
	// Test that relevance matching is case-insensitive
	testCases := []struct {
		name     string
		subject  string
		keyword  string
		expected int
	}{
		{"MOON", "MOON PROJECT", "moon", 1},        // Exact match, different case
		{"moon", "Moon Project", "MOON", 2},        // Prefix match, different case
		{"project", "The MOON Project", "moon", 3}, // Substring match, different case
		{"test", "Test Repository", "TEST", 1},     // Exact match, different case
	}

	for _, tc := range testCases {
		t.Run(tc.name+"_"+tc.keyword, func(t *testing.T) {
			score := calculateMockRelevanceScore(tc.name, tc.subject, tc.keyword)
			assert.Equal(t, tc.expected, score,
				"Case-insensitive matching failed for name=%s, subject=%s, keyword=%s",
				tc.name, tc.subject, tc.keyword)
		})
	}
}

func TestMultipleKeywordRelevance(t *testing.T) {
	// Test relevance scoring with multiple comma-separated keywords
	testRepo := MockRepository{Name: "moon-landing-project", Subject: "Moon Landing Project"}

	testCases := []struct {
		keywords      string
		expectedScore int
		description   string
	}{
		{"moon", 2, "single prefix match (Moon Landing Project)"},
		{"moon,landing", 5, "prefix + exact match (2+3)"},
		{"moon,landing,project", 8, "prefix + exact + exact match (2+3+3)"},
		{"space,moon", 6, "no match + prefix match (4+2)"},
		{"apollo,lunar", 8, "no match + no match (4+4)"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			keywords := strings.Split(tc.keywords, ",")
			totalScore := 0
			for _, keyword := range keywords {
				keyword = strings.TrimSpace(keyword)
				if keyword != "" {
					totalScore += calculateMockRelevanceScore(testRepo.Name, testRepo.Subject, keyword)
				}
			}
			assert.Equal(t, tc.expectedScore, totalScore, tc.description)
		})
	}
}

func TestSearchOptionsWithScoreSorting(t *testing.T) {
	unittest.PrepareTestEnv(t)

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	// Test various search options with score sorting
	testCases := []struct {
		name    string
		opts    repo_model.SearchRepoOptions
		orderBy db.SearchOrderBy
	}{
		{
			name: "ScoreSortingWithKeyword",
			opts: repo_model.SearchRepoOptions{
				ListOptions: db.ListOptions{PageSize: 5},
				Actor:       user,
				Keyword:     "test",
				Private:     true,
			},
			orderBy: repo_model.OrderByFlatMap["score"],
		},
		{
			name: "ReverseScoreSortingWithKeyword",
			opts: repo_model.SearchRepoOptions{
				ListOptions: db.ListOptions{PageSize: 5},
				Actor:       user,
				Keyword:     "repo",
				Private:     true,
			},
			orderBy: repo_model.OrderByFlatMap["reversescore"],
		},
		{
			name: "ScoreSortingWithoutKeyword",
			opts: repo_model.SearchRepoOptions{
				ListOptions: db.ListOptions{PageSize: 5},
				Actor:       user,
				Keyword:     "",
				Private:     true,
			},
			orderBy: db.SearchOrderByAlphabetically, // Use alphabetical when no keyword
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.opts.OrderBy = tc.orderBy

			repos, count, err := repo_model.SearchRepository(context.Background(), tc.opts)
			require.NoError(t, err, "SearchRepository should not return an error")

			if tc.opts.Keyword != "" {
				assert.Positive(t, count, "Should find repositories with keyword search")
			}

			// Verify that we get valid repository results
			for _, repo := range repos {
				assert.NotEmpty(t, repo.Name, "Repository should have a name")
				assert.NotNil(t, repo.Owner, "Repository should have an owner")
			}
		})
	}
}

// Helper function to sort repositories by relevance score
func sortByScore(repos []MockRepository, ascending bool) {
	sort.Slice(repos, func(i, j int) bool {
		if ascending {
			if repos[i].Score == repos[j].Score {
				// Secondary sort by display name alphabetically
				displayI := repos[i].Subject
				if displayI == "" {
					displayI = repos[i].Name
				}
				displayJ := repos[j].Subject
				if displayJ == "" {
					displayJ = repos[j].Name
				}
				return strings.ToLower(displayI) < strings.ToLower(displayJ)
			}
			return repos[i].Score < repos[j].Score
		}
		if repos[i].Score == repos[j].Score {
			// Secondary sort by display name alphabetically
			displayI := repos[i].Subject
			if displayI == "" {
				displayI = repos[i].Name
			}
			displayJ := repos[j].Subject
			if displayJ == "" {
				displayJ = repos[j].Name
			}
			return strings.ToLower(displayI) < strings.ToLower(displayJ)
		}
		return repos[i].Score > repos[j].Score
	})
}

func TestAPIEndpointCompatibility(t *testing.T) {
	// Test that the new sorting options work with the search infrastructure
	// This verifies that OrderByFlatMap integration works correctly

	// Test that score and reversescore exist in OrderByFlatMap
	scoreOrder, exists := repo_model.OrderByFlatMap["score"]
	assert.True(t, exists, "score should exist in OrderByFlatMap for API compatibility")
	assert.NotEmpty(t, scoreOrder, "score order should not be empty")

	reverseScoreOrder, exists := repo_model.OrderByFlatMap["reversescore"]
	assert.True(t, exists, "reversescore should exist in OrderByFlatMap for API compatibility")
	assert.NotEmpty(t, reverseScoreOrder, "reversescore order should not be empty")

	// Test that the SQL strings are valid
	assert.Contains(t, string(scoreOrder), "relevance_score", "score order should contain relevance_score")
	assert.Contains(t, string(reverseScoreOrder), "relevance_score", "reversescore order should contain relevance_score")

	// Test that they map to different SQL (one ASC, one DESC for relevance_score)
	assert.NotEqual(t, scoreOrder, reverseScoreOrder, "score and reversescore should have different SQL")
}
