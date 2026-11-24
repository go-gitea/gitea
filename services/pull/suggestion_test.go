// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"testing"

	"gitea.dev/modules/git"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSpliceCodeSuggestions(t *testing.T) {
	for _, test := range []struct {
		name, content, suggestion, want string
		start, end                      int64
	}{
		{"ReplaceCRLFRange", "before\r\nold\r\nold\r\nafter\r\n", "new\n", "before\r\nnew\r\nafter\r\n", 2, 3},
		{"NoFinalNewline", "before\r\nold", "new\nnext\n", "before\r\nnew\r\nnext", 2, 2},
		{"DeleteLastLine", "before\nold", "", "before\n", 2, 2},
		{"DeleteWholeFile", "old\n", "", "", 1, 1},
		{"TrailingBlankLine", "old\n", "new\n\n", "new\n\n", 1, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			content := []byte(test.content)
			edit, err := validateSuggestionLines(content, content, test.start, test.end, test.suggestion)
			require.NoError(t, err)
			result, err := spliceCodeSuggestions(content, []codeSuggestionEdit{edit})
			require.NoError(t, err)
			assert.Equal(t, test.want, string(result))
		})
	}
}

func TestSpliceCodeSuggestionsRejectsOverlap(t *testing.T) {
	_, err := spliceCodeSuggestions([]byte("first\nsecond\nthird\n"), []codeSuggestionEdit{
		{start: 6, end: 19, replacement: []byte("last\n")},
		{start: 0, end: 13, replacement: []byte("first\n")},
	})
	assert.ErrorIs(t, err, codeSuggestionRejection(codeSuggestionReasonPrefix+"overlap"))
}

func TestValidateSuggestionLines(t *testing.T) {
	const original = "before\r\nold\r\nafter\r\n"
	for _, test := range []struct {
		name, head, replacement, reason string
	}{
		{"ChangedLine", "before\r\nchanged\r\nafter\r\n", "new\n", "outdated"},
		{"OutOfBounds", "before\r\n", "new\n", "outdated"},
		{"Identical", original, "old\n", "no_changes"},
		{"BinaryReplacement", original, "new\x00\n", "unsupported_file"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := validateSuggestionLines([]byte(original), []byte(test.head), 2, 2, test.replacement)
			require.ErrorIs(t, err, codeSuggestionRejection(codeSuggestionReasonPrefix+test.reason))
			assert.ErrorIs(t, err, util.ErrUnprocessableContent)
		})
	}
}

func TestCodeSuggestionAnchorMoved(t *testing.T) {
	for _, test := range []struct {
		name string
		hunk git.FileDiffHunk
		want bool
	}{
		{"InsertBefore", git.FileDiffHunk{OldStart: 1, NewStart: 1, NewCount: 2}, true},
		{"InsertAfter", git.FileDiffHunk{OldStart: 5, NewStart: 5, NewCount: 1}, false},
		{"ReplaceBeforeWithoutShift", git.FileDiffHunk{OldStart: 1, OldCount: 1, NewStart: 1, NewCount: 1}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, codeSuggestionAnchorMoved([]git.FileDiffHunk{test.hunk}, 4, 4))
		})
	}
}
