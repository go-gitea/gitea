// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProcessNodeAttrID_HTMLHeadingWithoutID(t *testing.T) {
	// Test that HTML headings without id get an auto-generated id from their text content
	// when EnableHeadingIDGeneration is true (for repo files and wiki pages)
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "h1 without id",
			input:    `<h1>Heading without ID</h1>`,
			expected: `<h1 id="user-content-heading-without-id">Heading without ID</h1>`,
		},
		{
			name:     "h2 without id",
			input:    `<h2>Another Heading</h2>`,
			expected: `<h2 id="user-content-another-heading">Another Heading</h2>`,
		},
		{
			name:     "h3 without id",
			input:    `<h3>Third Level</h3>`,
			expected: `<h3 id="user-content-third-level">Third Level</h3>`,
		},
		{
			name:     "h1 with existing id should keep it",
			input:    `<h1 id="my-custom-id">Heading with ID</h1>`,
			expected: `<h1 id="user-content-my-custom-id">Heading with ID</h1>`,
		},
		{
			name:     "h1 with user-content prefix should not double prefix",
			input:    `<h1 id="user-content-already-prefixed">Already Prefixed</h1>`,
			expected: `<h1 id="user-content-already-prefixed">Already Prefixed</h1>`,
		},
		{
			name:     "heading with special characters",
			input:    `<h1>What is Wine Staging?</h1>`,
			expected: `<h1 id="user-content-what-is-wine-staging">What is Wine Staging?</h1>`,
		},
		{
			name:     "heading with nested elements",
			input:    `<h2><strong>Bold</strong> and <em>Italic</em></h2>`,
			expected: `<h2 id="user-content-bold-and-italic"><strong>Bold</strong> and <em>Italic</em></h2>`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result strings.Builder
			ctx := NewTestRenderContext().WithEnableHeadingIDGeneration(true)
			err := PostProcessDefault(ctx, strings.NewReader(tc.input), &result)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, strings.TrimSpace(result.String()))
		})
	}
}

func TestProcessNodeAttrID_SkipHeadingIDForComments(t *testing.T) {
	// Test that HTML headings in comment-like contexts (issue comments)
	// do NOT get auto-generated IDs to avoid duplicate IDs on pages with multiple documents.
	// This is controlled by EnableHeadingIDGeneration which defaults to false.
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "h1 without id in comment context",
			input:    `<h1>Heading without ID</h1>`,
			expected: `<h1>Heading without ID</h1>`,
		},
		{
			name:     "h2 without id in comment context",
			input:    `<h2>Another Heading</h2>`,
			expected: `<h2>Another Heading</h2>`,
		},
		{
			name:     "h1 with existing id should still be prefixed",
			input:    `<h1 id="my-custom-id">Heading with ID</h1>`,
			expected: `<h1 id="user-content-my-custom-id">Heading with ID</h1>`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result strings.Builder
			// Default context without EnableHeadingIDGeneration (simulates comment rendering)
			err := PostProcessDefault(NewTestRenderContext(), strings.NewReader(tc.input), &result)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, strings.TrimSpace(result.String()))
		})
	}
}
