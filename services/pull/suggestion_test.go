// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseSuggestionBlocks(t *testing.T) {
	input := "before\n```suggestion\nfoo\nbar\n```\nmid\n```go\nignore\n```\n```suggestion\nbaz\n```\nafter"
	blocks := ParseSuggestionBlocks(input)
	if assert.Len(t, blocks, 2) {
		assert.Equal(t, "foo\nbar", blocks[0].Content)
		assert.Equal(t, "baz", blocks[1].Content)
	}
}

func TestBuildSuggestionPatch(t *testing.T) {
	content := "a\nb\nc\nd\n"
	patch, err := BuildSuggestionPatch("file.txt", content, 2, 3, "x\ny", 1)
	assert.NoError(t, err)
	expected := "" +
		"diff --git a/file.txt b/file.txt\n" +
		"--- a/file.txt\n" +
		"+++ b/file.txt\n" +
		"@@ -1,4 +1,4 @@\n" +
		" a\n" +
		"-b\n" +
		"-c\n" +
		"+x\n" +
		"+y\n" +
		" d\n"
	assert.Equal(t, expected, patch)
}
