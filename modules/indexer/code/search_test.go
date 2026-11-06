// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package code

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildResultFragments(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		content  string
		matches  []MatchRange
		expected []*ResultFragment
	}{
		{name: "filename only", content: "no content matches"},
		{name: "context", content: "omit\nbefore\nhit\nafter\nomit", matches: []MatchRange{{Start: 12, End: 15}}, expected: []*ResultFragment{{StartLine: 2, Content: "before\nhit\nafter\n", Matches: []MatchRange{{Start: 7, End: 10}}}}},
		{name: "overlapping context", content: "0\n1\n2\n3\n4\n5", matches: []MatchRange{{Start: 2, End: 3}, {Start: 6, End: 7}}, expected: []*ResultFragment{{StartLine: 1, Content: "0\n1\n2\n3\n4\n", Matches: []MatchRange{{Start: 2, End: 3}, {Start: 6, End: 7}}}}},
		{name: "separate blocks", content: "0\n1\n2\n3\n4\n5\n6", matches: []MatchRange{{Start: 2, End: 3}, {Start: 10, End: 11}}, expected: []*ResultFragment{{StartLine: 1, Content: "0\n1\n2\n", Matches: []MatchRange{{Start: 2, End: 3}}}, {StartLine: 5, Content: "4\n5\n6", Matches: []MatchRange{{Start: 2, End: 3}}}}},
		{name: "first matching rune crosses fragment boundary", content: strings.Repeat("a", MaxResultFragmentSize-2) + "\n界", matches: []MatchRange{{Start: MaxResultFragmentSize - 1, End: MaxResultFragmentSize + 2}}, expected: []*ResultFragment{{StartLine: 2, Content: "界", Matches: []MatchRange{{Start: 0, End: 3}}}}},
		{name: "matches on long line", content: "alpha" + strings.Repeat("x", 5000) + "beta", matches: []MatchRange{{Start: 0, End: 5}, {Start: 5005, End: 5009}}, expected: []*ResultFragment{{StartLine: 1, Content: "alpha" + strings.Repeat("x", MaxResultFragmentSize-5), Matches: []MatchRange{{Start: 0, End: 5}}}, {StartLine: 1, Content: "beta", Matches: []MatchRange{{Start: 0, End: 4}}}}},
		{name: "overlapping bounded context", content: "a\nhit\n" + strings.Repeat("x", 4093) + "other", matches: []MatchRange{{Start: 2, End: 5}, {Start: 4099, End: 4104}}, expected: []*ResultFragment{{StartLine: 1, Content: "a\nhit\n" + strings.Repeat("x", MaxResultFragmentSize-6), Matches: []MatchRange{{Start: 2, End: 5}}}, {StartLine: 3, Content: "xxxother", Matches: []MatchRange{{Start: 3, End: 8}}}}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.expected, BuildResultFragments(testCase.content, testCase.matches))
		})
	}

	t.Run("fragment cap", func(t *testing.T) {
		content := strings.Repeat("hit\ncontext\ngap\ncontext\n", MaxResultFragments+1)
		var matches []MatchRange
		for i := range MaxResultFragments + 1 {
			matches = append(matches, MatchRange{Start: i * 24, End: i*24 + 3})
		}
		fragments := BuildResultFragments(content, matches)
		require.Len(t, fragments, MaxResultFragments)
		assert.Equal(t, &ResultFragment{StartLine: 1, Content: "hit\ncontext\n", Matches: []MatchRange{{Start: 0, End: 3}}}, fragments[0])
		for i, fragment := range fragments[1:] {
			assert.Equal(t, &ResultFragment{StartLine: (i + 1) * 4, Content: "context\nhit\ncontext\n", Matches: []MatchRange{{Start: 8, End: 11}}}, fragment)
		}
	})
}

func TestHighlightSearchResultCodeMatches(t *testing.T) {
	code := "x := \"a<b\"\r\nfoo()"
	lines := HighlightSearchResultCode("main.go", "", []int{1, 2}, code, MatchRange{Start: 2, End: 9}, MatchRange{Start: 12, End: 15})
	require.Len(t, lines, 2)
	assert.Equal(t, `<span class="nx">x</span><span class="w"> </span><span class="o"><mark>:=</mark></span><span class="w"><mark> </mark></span><span class="s"><mark>&#34;a&lt;b</mark>&#34;</span><span class="w">`+"\n</span>", string(lines[0].FormattedContent))
	assert.Equal(t, `<span class="nf"><mark>foo</mark></span><span class="p">(</span><span class="p">)</span>`, string(lines[1].FormattedContent))
}
