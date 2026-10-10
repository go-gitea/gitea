// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"strings"
	"testing"

	"gitea.dev/modules/setting"
	testModule "gitea.dev/modules/test"

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

func TestVisitNodeSource(t *testing.T) {
	defer testModule.MockVariableValue(&setting.Camo.Enabled, true)()
	defer testModule.MockVariableValue(&setting.Camo.Always, true)()
	defer testModule.MockVariableValue(&setting.Camo.ServerURL, "https://image.proxy")()
	defer testModule.MockVariableValue(&setting.Camo.HMACKey, "key")()

	test := func(input, expected string) {
		t.Helper()
		var result strings.Builder
		err := PostProcessDefault(NewTestRenderContext("/base"), strings.NewReader(input), &result)
		assert.NoError(t, err)
		assert.Equal(t, expected, strings.TrimSpace(result.String()))
	}

	// a relative candidate resolves against the media link, just like "img" and "video" do
	test(`<picture><source media="(prefers-color-scheme: dark)" srcset="dark.svg"></picture>`,
		`<picture><source media="(prefers-color-scheme: dark)" srcset="/base/dark.svg"/></picture>`)
	// an external candidate goes through the media proxy instead of being fetched by the browser
	test(`<source srcset="http://example.com/img.jpg">`,
		`<source srcset="https://image.proxy/ot-IzsO3Va2BTuasnEa3Ddw3XC8/aHR0cDovL2V4YW1wbGUuY29tL2ltZy5qcGc"/>`)
}

func TestResolveSrcSetLinks(t *testing.T) {
	resolve := func(link string) string { return "/base/" + link }
	cases := []struct{ input, expected string }{
		{"a.png", "/base/a.png"},
		{" a.png 1x, b.png 2x ", " /base/a.png 1x, /base/b.png 2x "},
		{"a.png, b.png", "/base/a.png, /base/b.png"},
		{"a.png,b.png", "/base/a.png,b.png"}, // only a trailing comma closes a candidate
		{"a,b.png 1x", "/base/a,b.png 1x"},
		{"", ""},
		{" ,, ", " ,, "},
	}
	for _, c := range cases {
		assert.Equal(t, c.expected, resolveSrcSetLinks(c.input, resolve), "input: %q", c.input)
	}
}
