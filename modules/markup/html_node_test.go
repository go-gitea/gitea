// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProcessNodeAttrID_HTMLHeadingWithoutID(t *testing.T) {
	testCases := []struct {
		name              string
		generateHeadingID bool
		input, expected   string
	}{
		// Test that HTML headings without id get an auto-generated id from their text content
		// when EnableHeadingIDGeneration is true (for repo files and wiki pages)
		{
			"h1 without id", true,
			`<h1>Heading without ID</h1>`,
			`<h1 id="user-content-heading-without-id">Heading without ID</h1>`,
		},
		{
			"h2 without id", true,
			`<h2>Another Heading</h2>`,
			`<h2 id="user-content-another-heading">Another Heading</h2>`,
		},
		{
			"h3 without id", true,
			`<h3>Third Level</h3>`,
			`<h3 id="user-content-third-level">Third Level</h3>`,
		},
		{
			"h1 with existing id should keep it", true,
			`<h1 id="my-custom-id">Heading with ID</h1>`,
			`<h1 id="user-content-my-custom-id">Heading with ID</h1>`,
		},
		{
			"h1 with user-content prefix should not double prefix", true,
			`<h1 id="user-content-already-prefixed">Already Prefixed</h1>`,
			`<h1 id="user-content-already-prefixed">Already Prefixed</h1>`,
		},
		{
			"heading with special characters", true,
			`<h1>What is Wine Staging?</h1>`,
			`<h1 id="user-content-what-is-wine-staging">What is Wine Staging?</h1>`,
		},
		{
			"heading with nested elements", true,
			`<h2><strong>Bold</strong> and <em>Italic</em></h2>`,
			`<h2 id="user-content-bold-and-italic"><strong>Bold</strong> and <em>Italic</em></h2>`,
		},
		// Test that HTML headings in comment-like contexts (issue comments)
		// do NOT get auto-generated IDs to avoid duplicate IDs on pages with multiple documents.
		// This is controlled by EnableHeadingIDGeneration which defaults to false.
		// Default context without EnableHeadingIDGeneration (simulates comment rendering)
		{
			"h1 without id in comment context", false,
			`<h1>Heading without ID</h1>`,
			`<h1>Heading without ID</h1>`,
		},
		{
			"h2 without id in comment context", false,
			`<h2>Another Heading</h2>`,
			`<h2>Another Heading</h2>`,
		},
		{
			"h1 with existing id should still be prefixed", false,
			`<h1 id="my-custom-id">Heading with ID</h1>`,
			`<h1 id="user-content-my-custom-id">Heading with ID</h1>`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result strings.Builder
			ctx := NewTestRenderContext()
			if tc.generateHeadingID {
				ctx.WithEnableHeadingIDGeneration(true)
			}
			err := PostProcessDefault(ctx, strings.NewReader(tc.input), &result)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, strings.TrimSpace(result.String()))
		})
	}
}
