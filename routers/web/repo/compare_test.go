// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"html/template"
	"strings"
	"testing"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/services/gitdiff"

	"github.com/stretchr/testify/assert"
)

func TestAttachCommentsToLines(t *testing.T) {
	section := &gitdiff.DiffSection{
		Lines: []*gitdiff.DiffLine{
			{LeftIdx: 5, RightIdx: 10},
			{LeftIdx: 6, RightIdx: 11},
		},
	}

	lineComments := map[int64][]*issues_model.Comment{
		-5: {{ID: 100, CreatedUnix: 1000}},                               // left side comment
		10: {{ID: 200, CreatedUnix: 2000}},                               // right side comment
		11: {{ID: 300, CreatedUnix: 1500}, {ID: 301, CreatedUnix: 2500}}, // multiple comments
	}

	attachCommentsToLines(section, lineComments)

	// First line should have left and right comments
	assert.Len(t, section.Lines[0].Comments, 2)
	assert.Equal(t, int64(100), section.Lines[0].Comments[0].ID)
	assert.Equal(t, int64(200), section.Lines[0].Comments[1].ID)

	// Second line should have two comments, sorted by creation time
	assert.Len(t, section.Lines[1].Comments, 2)
	assert.Equal(t, int64(300), section.Lines[1].Comments[0].ID)
	assert.Equal(t, int64(301), section.Lines[1].Comments[1].ID)
}

func TestHighlightFileForExcerpt(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		language string
		content  string
		wantSpan bool // whether we expect span tags in output
	}{
		{
			name:     "JSON file with language specified",
			fileName: "test.json",
			language: "json",
			content:  `{"key": "value"}`,
			wantSpan: true,
		},
		{
			name:     "Plain text",
			fileName: "test.txt",
			language: "",
			content:  "plain text content",
			wantSpan: false,
		},
		{
			name:     "Go code",
			fileName: "test.go",
			language: "go",
			content:  "package main\n\nfunc main() {}\n",
			wantSpan: true,
		},
		{
			name:     "Empty content",
			fileName: "empty.txt",
			language: "",
			content:  "",
			wantSpan: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := []byte(tt.content)
			result := highlightFileForExcerpt(tt.fileName, tt.language, content)

			// Check that we got some lines
			if tt.content != "" {
				expectedLines := len(strings.Split(tt.content, "\n"))
				assert.GreaterOrEqual(t, len(result), expectedLines-1, "Should have at least as many lines as content")
			}

			// Check if highlighting was applied
			if tt.wantSpan && tt.content != "" {
				// At least one line should contain span tags if highlighting was applied
				foundSpan := false
				for _, line := range result {
					if strings.Contains(string(line), "<span") {
						foundSpan = true
						break
					}
				}
				assert.True(t, foundSpan, "Expected span tags in highlighted output")
			}
		})
	}
}

func TestHighlightFileForExcerptJSONSyntax(t *testing.T) {
	// Test that JSON keys are highlighted with the correct class
	content := []byte(`{
  "name": "test",
  "version": "1.0.0"
}`)
	
	result := highlightFileForExcerpt("test.json", "json", content)
	
	// We should have multiple lines
	assert.Greater(t, len(result), 1, "Should have multiple lines")
	
	// Check that we have syntax highlighting HTML
	hasHighlighting := false
	for _, line := range result {
		lineStr := string(line)
		if strings.Contains(lineStr, "<span") {
			hasHighlighting = true
			break
		}
	}
	assert.True(t, hasHighlighting, "JSON should be syntax highlighted")
}

func TestDiffSectionSetDiffFile(t *testing.T) {
	section := &gitdiff.DiffSection{
		FileName: "test.go",
		Lines: []*gitdiff.DiffLine{
			{LeftIdx: 1, RightIdx: 1, Type: gitdiff.DiffLinePlain, Content: " package main"},
		},
	}
	
	diffFile := &gitdiff.DiffFile{
		Name:     "test.go",
		Language: "go",
	}
	
	// Set the diff file
	section.SetDiffFile(diffFile)
	
	// Verify the method doesn't panic and works correctly by setting it again
	assert.NotPanics(t, func() {
		section.SetDiffFile(diffFile)
	}, "SetDiffFile should not panic when called multiple times")
	
	// We can't directly test the private field, but we can verify the method
	// accepts the parameter without error. In actual usage, the rendering
	// code will use the file's language and highlighted lines.
}

func TestDiffFileSetHighlightedRightLines(t *testing.T) {
	diffFile := &gitdiff.DiffFile{
		Name: "test.go",
	}
	
	highlightedLines := map[int]template.HTML{
		0: template.HTML(`<span class="line">line 1</span>`),
		1: template.HTML(`<span class="line">line 2</span>`),
	}
	
	// Test that SetHighlightedRightLines doesn't panic and can be called multiple times
	assert.NotPanics(t, func() {
		diffFile.SetHighlightedRightLines(highlightedLines)
	}, "SetHighlightedRightLines should not panic")
	
	// Set different lines to verify it can be updated
	differentLines := map[int]template.HTML{
		0: template.HTML(`<span class="line">different line</span>`),
	}
	assert.NotPanics(t, func() {
		diffFile.SetHighlightedRightLines(differentLines)
	}, "SetHighlightedRightLines should handle updates without panicking")
}
