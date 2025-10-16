// Copyright 2025 okTurtles Foundation. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildRelevanceScoreSQL_InputValidation(t *testing.T) {
	tests := []struct {
		name        string
		keyword     string
		expectZero  bool
		description string
	}{
		{
			name:        "Normal single keyword",
			keyword:     "moon",
			expectZero:  false,
			description: "Should accept normal single keyword",
		},
		{
			name:        "Normal multiple keywords",
			keyword:     "moon,landing,apollo",
			expectZero:  false,
			description: "Should accept multiple comma-separated keywords",
		},
		{
			name:        "Empty string",
			keyword:     "",
			expectZero:  true,
			description: "Should return '0' for empty string",
		},
		{
			name:        "Only whitespace",
			keyword:     "   ",
			expectZero:  true,
			description: "Should return '0' for whitespace-only string",
		},
		{
			name:        "Excessive keywords (>10)",
			keyword:     "a,b,c,d,e,f,g,h,i,j,k,l,m,n,o",
			expectZero:  false,
			description: "Should truncate to 10 keywords",
		},
		{
			name:        "Very long keyword (>100 chars)",
			keyword:     strings.Repeat("a", 150),
			expectZero:  false,
			description: "Should truncate individual keyword to 100 chars",
		},
		{
			name:        "Very long total string (>500 chars)",
			keyword:     strings.Repeat("keyword,", 100),
			expectZero:  false,
			description: "Should truncate total string to 500 chars",
		},
		{
			name:        "Keywords with whitespace",
			keyword:     " moon , landing , apollo ",
			expectZero:  false,
			description: "Should trim whitespace from keywords",
		},
		{
			name:        "Empty keywords in list",
			keyword:     "moon,,landing,,,apollo",
			expectZero:  false,
			description: "Should skip empty keywords",
		},
		{
			name:        "All empty keywords",
			keyword:     ",,,,,",
			expectZero:  true,
			description: "Should return '0' when all keywords are empty",
		},
		{
			name:        "Unicode characters",
			keyword:     "café,naïve,Zürich",
			expectZero:  false,
			description: "Should accept Unicode characters",
		},
		{
			name:        "Special SQL characters",
			keyword:     "test';DROP TABLE--",
			expectZero:  false,
			description: "Should accept special characters (placeholders prevent injection)",
		},
		{
			name:        "Mixed valid and empty",
			keyword:     "moon, , landing, , apollo",
			expectZero:  false,
			description: "Should skip empty keywords but keep valid ones",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildRelevanceScoreSQL(tt.keyword)

			if tt.expectZero {
				assert.Equal(t, "0", result, tt.description)
			} else {
				assert.NotEqual(t, "0", result, tt.description)
				// Verify the result contains SQL CASE statements
				assert.Contains(t, result, "CASE", "Result should contain SQL CASE statement")
				assert.Contains(t, result, "COALESCE", "Result should contain COALESCE function")
			}
		})
	}
}

func TestBuildRelevanceScoreSQL_KeywordLimits(t *testing.T) {
	t.Run("Exactly 10 keywords", func(t *testing.T) {
		keyword := "a,b,c,d,e,f,g,h,i,j"
		result := buildRelevanceScoreSQL(keyword)
		assert.NotEqual(t, "0", result)
		// Count the number of CASE statements (should be 10)
		caseCount := strings.Count(result, "CASE")
		assert.Equal(t, 10, caseCount, "Should have exactly 10 CASE statements for 10 keywords")
	})

	t.Run("More than 10 keywords", func(t *testing.T) {
		keyword := "a,b,c,d,e,f,g,h,i,j,k,l,m,n,o"
		result := buildRelevanceScoreSQL(keyword)
		assert.NotEqual(t, "0", result)
		// Count the number of CASE statements (should be limited to 10)
		caseCount := strings.Count(result, "CASE")
		assert.Equal(t, 10, caseCount, "Should limit to 10 CASE statements even with more keywords")
	})

	t.Run("Individual keyword length limit", func(t *testing.T) {
		// Create a keyword longer than 100 chars
		longKeyword := strings.Repeat("a", 150)
		result := buildRelevanceScoreSQL(longKeyword)
		assert.NotEqual(t, "0", result)
		// The function should still work, just with truncated keyword
		assert.Contains(t, result, "CASE")
	})

	t.Run("Total string length limit", func(t *testing.T) {
		// Create a string longer than 500 chars
		longString := strings.Repeat("keyword,", 100) // ~800 chars
		result := buildRelevanceScoreSQL(longString)
		assert.NotEqual(t, "0", result)
		// The function should still work, just with truncated string
		assert.Contains(t, result, "CASE")
	})
}

func TestBuildRelevanceScoreSQL_EdgeCases(t *testing.T) {
	t.Run("Single character keyword", func(t *testing.T) {
		result := buildRelevanceScoreSQL("a")
		assert.NotEqual(t, "0", result)
		assert.Contains(t, result, "CASE")
	})

	t.Run("Keyword with only commas", func(t *testing.T) {
		result := buildRelevanceScoreSQL(",,,")
		assert.Equal(t, "0", result, "Should return '0' for only commas")
	})

	t.Run("Keyword with spaces and commas", func(t *testing.T) {
		result := buildRelevanceScoreSQL(" , , , ")
		assert.Equal(t, "0", result, "Should return '0' for only spaces and commas")
	})

	t.Run("Mixed whitespace types", func(t *testing.T) {
		result := buildRelevanceScoreSQL("\t\n\r moon \t\n\r")
		assert.NotEqual(t, "0", result)
		assert.Contains(t, result, "CASE")
	})

	t.Run("Newlines in keywords", func(t *testing.T) {
		result := buildRelevanceScoreSQL("moon\nlanding")
		assert.NotEqual(t, "0", result)
		// Should treat as single keyword (no comma separator)
		caseCount := strings.Count(result, "CASE")
		assert.Equal(t, 1, caseCount)
	})
}

func TestBuildRelevanceScoreSQL_SecurityCases(t *testing.T) {
	t.Run("SQL injection attempt - single quote", func(t *testing.T) {
		result := buildRelevanceScoreSQL("test' OR '1'='1")
		assert.NotEqual(t, "0", result)
		// Should still generate valid SQL (placeholders prevent injection)
		assert.Contains(t, result, "CASE")
	})

	t.Run("SQL injection attempt - comment", func(t *testing.T) {
		result := buildRelevanceScoreSQL("test--")
		assert.NotEqual(t, "0", result)
		assert.Contains(t, result, "CASE")
	})

	t.Run("SQL injection attempt - union", func(t *testing.T) {
		result := buildRelevanceScoreSQL("test UNION SELECT")
		assert.NotEqual(t, "0", result)
		// Should treat as single keyword (no comma)
		caseCount := strings.Count(result, "CASE")
		assert.Equal(t, 1, caseCount)
	})

	t.Run("DoS attempt - many keywords", func(t *testing.T) {
		// Try to create 1000 keywords
		keywords := make([]string, 1000)
		for i := range keywords {
			keywords[i] = "keyword"
		}
		longKeyword := strings.Join(keywords, ",")
		result := buildRelevanceScoreSQL(longKeyword)
		assert.NotEqual(t, "0", result)
		// Should be limited to 10 keywords
		caseCount := strings.Count(result, "CASE")
		assert.Equal(t, 10, caseCount, "Should limit to 10 keywords to prevent DoS")
	})

	t.Run("DoS attempt - very long string", func(t *testing.T) {
		// Try to create a 10KB string
		longString := strings.Repeat("a", 10000)
		result := buildRelevanceScoreSQL(longString)
		assert.NotEqual(t, "0", result)
		// Should still work (truncated to 500 chars)
		assert.Contains(t, result, "CASE")
	})
}

func TestBuildRelevanceScoreSQL_OutputFormat(t *testing.T) {
	t.Run("Single keyword output format", func(t *testing.T) {
		result := buildRelevanceScoreSQL("moon")
		// Should be wrapped in parentheses
		assert.True(t, strings.HasPrefix(result, "("), "Result should start with (")
		assert.True(t, strings.HasSuffix(result, ")"), "Result should end with )")
		// Should contain the scoring logic
		assert.Contains(t, result, "WHEN LOWER(COALESCE(subject.name, repository.name)) = ? THEN 1")
		assert.Contains(t, result, "WHEN LOWER(COALESCE(subject.name, repository.name)) LIKE ? THEN 2")
		assert.Contains(t, result, "WHEN LOWER(COALESCE(subject.name, repository.name)) LIKE ? THEN 3")
		assert.Contains(t, result, "ELSE 4")
	})

	t.Run("Multiple keywords output format", func(t *testing.T) {
		result := buildRelevanceScoreSQL("moon,landing")
		// Should contain addition operator for combining scores
		assert.Contains(t, result, "+", "Multiple keywords should be combined with +")
		// Should have 2 CASE statements
		caseCount := strings.Count(result, "CASE")
		assert.Equal(t, 2, caseCount)
	})
}

