// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitgrep

import (
	"testing"

	"gitea.dev/modules/git"
	code_indexer "gitea.dev/modules/indexer/code"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIndexSettingToGitGrepPathspecList(t *testing.T) {
	defer test.MockVariableValue(&setting.Indexer.IncludePatterns, setting.IndexerGlobFromString("a"))()
	defer test.MockVariableValue(&setting.Indexer.ExcludePatterns, setting.IndexerGlobFromString("b"))()
	assert.Equal(t, []string{":(glob,icase)a", ":(glob,exclude,icase)b"}, indexSettingToGitGrepPathspecList(""))
	assert.Equal(t, []string{":(glob,icase)a", ":(glob,exclude,icase)b"}, indexSettingToGitGrepPathspecList("x/y"))

	defer test.MockVariableValue(&setting.Indexer.IncludePatterns, nil)()
	assert.Equal(t, []string{":(literal)x/y/", ":(glob,exclude,icase)b"}, indexSettingToGitGrepPathspecList("x/y"))
}

func TestGrepMatchRanges(t *testing.T) {
	const content = "Foo bar\nfoo(x) --verbose"
	cases := []struct {
		keyword  string
		mode     git.GrepModeType
		expected []code_indexer.MatchRange
	}{
		{"foo", git.GrepModeExact, []code_indexer.MatchRange{{Start: 8, End: 11}}},
		{"foo(x)", git.GrepModeExact, []code_indexer.MatchRange{{Start: 8, End: 14}}},
		{"--verbose", git.GrepModeExact, []code_indexer.MatchRange{{Start: 15, End: 24}}},
		{"FOO bar", git.GrepModeWords, []code_indexer.MatchRange{{Start: 0, End: 3}, {Start: 4, End: 7}, {Start: 8, End: 11}}},
		{"f.o", git.GrepModeRegexp, []code_indexer.MatchRange{{Start: 8, End: 11}}},
	}
	for _, c := range cases {
		pattern, err := grepMatchPattern(c.keyword, c.mode)
		require.NoError(t, err)
		assert.Equal(t, c.expected, findMatchRanges(pattern, content), "keyword=%q mode=%s", c.keyword, c.mode)
	}

	_, err := grepMatchPattern(`(?<=a)b`, git.GrepModeRegexp) // PCRE lookbehind is not supported by Go
	assert.Error(t, err)
	assert.Nil(t, findMatchRanges(nil, content))
}
