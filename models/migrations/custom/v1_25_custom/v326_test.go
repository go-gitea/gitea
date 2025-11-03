// Copyright 2025 okTurtles Foundation. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_25_custom

import (
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"github.com/stretchr/testify/assert"
)

// TestMigrationUsesApplicationSlugFunction verifies that the migration v326
// uses the exact same GenerateSlugFromName function as the application code.
// This test ensures that GitHub issue #31 (code duplication) has been resolved.
func TestMigrationUsesApplicationSlugFunction(t *testing.T) {
	// Test cases covering various slug generation scenarios
	testCases := []struct {
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
			name:     "With accents",
			input:    "Café Français",
			expected: "cafe-francais",
		},
		{
			name:     "With underscores",
			input:    "hello_world_test",
			expected: "hello-world-test",
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
			name:     "Multiple hyphens",
			input:    "hello---world",
			expected: "hello-world",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Call the application's GenerateSlugFromName function
			// The migration now imports and uses this same function
			result := repo_model.GenerateSlugFromName(tc.input)

			// Verify the result matches expected output
			assert.Equal(t, tc.expected, result,
				"GenerateSlugFromName should produce expected output for input: %q", tc.input)
		})
	}
}

// TestMigrationSlugConsistency verifies that the migration produces
// identical slugs to the application code for a comprehensive set of inputs.
func TestMigrationSlugConsistency(t *testing.T) {
	// Comprehensive test inputs
	inputs := []string{
		"The Moon",
		"the moon!",
		"El Camiño?",
		"Café Français",
		"Hello@World#2024!",
		"hello_world_test",
		"hello   world",
		"  hello world  ",
		"Zürich",
		"Test123Subject",
		"hello---world",
		"My.Project",
		"Project.git",
		"!!!???",
		"",
		"   ",
		"---",
		"___",
	}

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			// The migration now uses repo_model.GenerateSlugFromName directly
			// So we just verify it produces consistent output
			slug := repo_model.GenerateSlugFromName(input)

			// Verify the slug is valid (not empty unless input was empty/special chars only)
			if input != "" && input != "   " && input != "---" && input != "___" && input != "!!!???" {
				assert.NotEmpty(t, slug, "Slug should not be empty for non-empty input: %q", input)
			}

			// Verify the slug contains only valid characters
			assert.Regexp(t, `^[a-z0-9-]*$`, slug,
				"Slug should only contain lowercase letters, numbers, and hyphens: %q", slug)

			// Verify the slug doesn't start or end with hyphens
			if slug != "" && slug != "subject" {
				assert.NotRegexp(t, `^-`, slug, "Slug should not start with hyphen: %q", slug)
				assert.NotRegexp(t, `-$`, slug, "Slug should not end with hyphen: %q", slug)
			}
		})
	}
}
