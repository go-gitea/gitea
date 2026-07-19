// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package bleve

import (
	"testing"

	"github.com/blevesearch/bleve/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRepoIndexerAnalyzerIndexesNumericTerms is a regression test for #37221:
// the repoIndexerAnalyzer used bleve's `letter` tokenizer, which only emits
// tokens made of unicode letters and silently drops digit runs entirely, so a
// purely-numeric search term (or the numeric part of "vlan 699") never
// matched anything. The lettersdigits tokenizer fixes this by treating digits
// as token characters too, while still splitting on punctuation like the old
// `letter` tokenizer did (see the sibling test below).
func TestRepoIndexerAnalyzerIndexesNumericTerms(t *testing.T) {
	mapping, err := generateBleveIndexMapping()
	require.NoError(t, err)

	index, err := bleve.New(t.TempDir(), mapping)
	require.NoError(t, err)
	defer index.Close()

	require.NoError(t, index.Index("1", &RepoIndexerData{
		RepoID:   1,
		CommitID: "0000000000000000000000000000000000000000",
		Content:  "vlan 699 configuration",
		Filename: "config.txt",
		Language: "Text",
	}))

	query := bleve.NewMatchQuery("699")
	query.FieldVal = "Content"
	result, err := index.Search(bleve.NewSearchRequest(query))
	require.NoError(t, err)
	assert.NotZero(t, result.Total, "purely-numeric search term should match indexed content containing it")
}

// TestRepoIndexerAnalyzerStillSplitsOnPunctuation guards against a naive fix
// for #37221 (e.g. switching to bleve's `unicode` tokenizer) that would
// glue "console.log" into a single token instead of "console" + "log",
// silently breaking the existing TestBleveIndexAndSearch/log case.
func TestRepoIndexerAnalyzerStillSplitsOnPunctuation(t *testing.T) {
	mapping, err := generateBleveIndexMapping()
	require.NoError(t, err)

	index, err := bleve.New(t.TempDir(), mapping)
	require.NoError(t, err)
	defer index.Close()

	require.NoError(t, index.Index("1", &RepoIndexerData{
		RepoID:   1,
		CommitID: "0000000000000000000000000000000000000000",
		Content:  `console.log("Hello, World!")`,
		Filename: "example.js",
		Language: "JavaScript",
	}))

	query := bleve.NewMatchQuery("log")
	query.FieldVal = "Content"
	result, err := index.Search(bleve.NewSearchRequest(query))
	require.NoError(t, err)
	assert.NotZero(t, result.Total, "\"log\" should match inside \"console.log(...)\" as its own token")
}

// TestRepoIndexerAnalyzerSegmentsCJKPerCharacter guards a regression a naive
// fix for #37221 would introduce: bleve's `[\p{L}\p{N}]+`-style regexp
// tokenizers treat CJK ideographs as ordinary letters and glue an entire CJK
// phrase into one token (worse than the pre-fix `letter` tokenizer for mixed
// CJK+digit content, since digits would previously split the run instead of
// merging into it). The lettersdigits tokenizer segments CJK ideograph/kana/
// hangul runes individually, matching how filenameIndexerAnalyzer's `unicode`
// tokenizer already treats CJK, so searching for a single CJK character still
// finds a multi-character CJK phrase containing it.
func TestRepoIndexerAnalyzerSegmentsCJKPerCharacter(t *testing.T) {
	mapping, err := generateBleveIndexMapping()
	require.NoError(t, err)

	index, err := bleve.New(t.TempDir(), mapping)
	require.NoError(t, err)
	defer index.Close()

	require.NoError(t, index.Index("1", &RepoIndexerData{
		RepoID:   1,
		CommitID: "0000000000000000000000000000000000000000",
		Content:  "变量123名称", // "variable123name" with CJK words either side of a digit run
		Filename: "example.txt",
		Language: "Text",
	}))

	for _, term := range []string{"变", "称", "123"} {
		query := bleve.NewMatchQuery(term)
		query.FieldVal = "Content"
		result, err := index.Search(bleve.NewSearchRequest(query))
		require.NoError(t, err)
		assert.NotZero(t, result.Total, "%q should match as its own token inside the mixed CJK+digit content", term)
	}
}
