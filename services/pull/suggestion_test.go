// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"testing"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/modules/cache"
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

func TestValidateSuggestionEditLineShifts(t *testing.T) {
	const path = "lines.txt"
	const original = "header\nrepeat\nbetween\nrepeat\nlast\n"
	repo, err := git.ForceFastImportWithInit(t.Context(), t.TempDir(), []git.FastImportCommit{
		{Ref: "refs/heads/anchor", Files: []git.FastImportFile{{Path: path, Content: original}}},
		{Ref: "refs/heads/shifted", Parent: ":1", Files: []git.FastImportFile{{Path: path, Content: "inserted\nextra\n" + original}}},
		{Ref: "refs/heads/after", Parent: ":1", Files: []git.FastImportFile{{Path: path, Content: "header\nrepeat\nbetween\nrepeat\nextra\nlast\n"}}},
		{Ref: "refs/heads/replaced", Parent: ":1", Files: []git.FastImportFile{{Path: path, Content: "HEADER\nrepeat\nbetween\nrepeat\nlast\n"}}},
	})
	require.NoError(t, err)
	gitRepo, err := git.OpenRepository(t.Context(), repo)
	require.NoError(t, err)
	defer gitRepo.Close()
	anchor, err := gitRepo.GetBranchCommitID(t.Context(), "anchor")
	require.NoError(t, err)
	for _, test := range []struct {
		branch string
		want   error
	}{
		{"shifted", codeSuggestionRejection(codeSuggestionReasonPrefix + "outdated")},
		{"after", nil},
		{"replaced", nil},
	} {
		t.Run(test.branch, func(t *testing.T) {
			head, err := gitRepo.GetBranchCommitID(t.Context(), test.branch)
			require.NoError(t, err)
			loader := &codeSuggestionLoader{
				pr: &issues_model.PullRequest{}, headRepo: gitRepo, headCommitID: head, cache: cache.NewEphemeralCache(),
			}
			_, err = loader.validateEdit(t.Context(), &issues_model.Comment{
				Line: 4, TreePath: path, CommentMetaData: &issues_model.CommentMetaData{CodeCommitID: anchor},
			}, "new\n")
			assert.ErrorIs(t, err, test.want)
		})
	}
}
