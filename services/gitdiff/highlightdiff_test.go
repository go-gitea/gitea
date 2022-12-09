// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitdiff

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/stretchr/testify/assert"
)

func TestDiffWithHighlight(t *testing.T) {
	hcd := newHighlightCodeDiff()
	diffs := hcd.diffWithHighlight(
		"main.v", "",
		"		run('<>')\n",
		"		run(db)\n",
	)

	expected := `		<span class="n">run</span><span class="o">(</span><span class="removed-code"><span class="k">&#39;</span><span class="o">&lt;</span><span class="o">&gt;</span><span class="k">&#39;</span></span><span class="o">)</span>`
	output := diffToHTML(nil, diffs, DiffLineDel)
	assert.Equal(t, expected, output)

	expected = `		<span class="n">run</span><span class="o">(</span><span class="added-code"><span class="n">db</span></span><span class="o">)</span>`
	output = diffToHTML(nil, diffs, DiffLineAdd)
	assert.Equal(t, expected, output)

	hcd = newHighlightCodeDiff()
	hcd.placeholderTokenMap['O'] = "<span>"
	hcd.placeholderTokenMap['C'] = "</span>"
	diff := diffmatchpatch.Diff{}

	diff.Text = "OC"
	hcd.recoverOneDiff(&diff)
	assert.Equal(t, "<span></span>", diff.Text)

	diff.Text = "O"
	hcd.recoverOneDiff(&diff)
	assert.Equal(t, "<span></span>", diff.Text)

	diff.Text = "C"
	hcd.recoverOneDiff(&diff)
	assert.Equal(t, "", diff.Text)
}

func TestDiffWithHighlightPlaceholder(t *testing.T) {
	hcd := newHighlightCodeDiff()
	diffs := hcd.diffWithHighlight(
		"main.js", "",
		"a='\U00100000'",
		"a='\U0010FFFD''",
	)
	assert.Equal(t, "", hcd.placeholderTokenMap[0x00100000])
	assert.Equal(t, "", hcd.placeholderTokenMap[0x0010FFFD])

	expected := fmt.Sprintf(`<span class="nx">a</span><span class="o">=</span><span class="s1">&#39;</span><span class="removed-code">%s</span>&#39;`, "\U00100000")
	output := diffToHTML(hcd.lineWrapperTags, diffs, DiffLineDel)
	assert.Equal(t, expected, output)

	hcd = newHighlightCodeDiff()
	diffs = hcd.diffWithHighlight(
		"main.js", "",
		"a='\U00100000'",
		"a='\U0010FFFD'",
	)
	expected = fmt.Sprintf(`<span class="nx">a</span><span class="o">=</span><span class="s1">&#39;</span><span class="added-code">%s</span>&#39;`, "\U0010FFFD")
	output = diffToHTML(nil, diffs, DiffLineAdd)
	assert.Equal(t, expected, output)
}

func TestDiffWithHighlightPlaceholderExhausted(t *testing.T) {
	hcd := newHighlightCodeDiff()
	hcd.placeholderMaxCount = 0
	diffs := hcd.diffWithHighlight(
		"main.js", "",
		"'",
		``,
	)
	output := diffToHTML(nil, diffs, DiffLineDel)
	expected := fmt.Sprintf(`<span class="removed-code">%s#39;</span>`, "\uFFFD")
	assert.Equal(t, expected, output)

	hcd = newHighlightCodeDiff()
	hcd.placeholderMaxCount = 0
	diffs = hcd.diffWithHighlight(
		"main.js", "",
		"a < b",
		"a > b",
	)
	output = diffToHTML(nil, diffs, DiffLineDel)
	expected = fmt.Sprintf(`a %s<span class="removed-code">l</span>t; b`, "\uFFFD")
	assert.Equal(t, expected, output)

	output = diffToHTML(nil, diffs, DiffLineAdd)
	expected = fmt.Sprintf(`a %s<span class="added-code">g</span>t; b`, "\uFFFD")
	assert.Equal(t, expected, output)
}

func TestDiffWithHighlightTagMatch(t *testing.T) {
	totalOverflow := 0
	for i := 0; i < 100; i++ {
		hcd := newHighlightCodeDiff()
		hcd.placeholderMaxCount = i
		diffs := hcd.diffWithHighlight(
			"main.js", "",
			"a='1'",
			"b='2'",
		)
		totalOverflow += hcd.placeholderOverflowCount

		output := diffToHTML(nil, diffs, DiffLineDel)
		c1 := strings.Count(output, "<span")
		c2 := strings.Count(output, "</span")
		assert.Equal(t, c1, c2)

		output = diffToHTML(nil, diffs, DiffLineAdd)
		c1 = strings.Count(output, "<span")
		c2 = strings.Count(output, "</span")
		assert.Equal(t, c1, c2)
	}
	assert.NotZero(t, totalOverflow)
}
