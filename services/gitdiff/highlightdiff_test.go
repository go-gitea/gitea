// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitdiff

import (
	"fmt"
	"testing"

	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/stretchr/testify/assert"
)

func TestDiffWithHighlight(t *testing.T) {
	hcd := newHighlightCodeDiff()
	diffs := hcd.diffWithHighlight(
		"		run('<>')\n",
		"		run(db)\n",
	)

	expected := "\t\trun(<span class=\"removed-code\">'<>'</></span>)\n"

	output := diffToHTML(nil, diffs, DiffLineDel)
	assert.Equal(t, expected, output)

	expected = `		run(<span class="added-code">db</span>)
`
	output = diffToHTML(nil, diffs, DiffLineAdd)
	assert.Equal(t, expected, output)

	hcd = newHighlightCodeDiff()
	hcd.placeholderTokenMap['O'] = "<span>"
	hcd.placeholderTokenMap['C'] = "</span>"
	diff := diffmatchpatch.Diff{}

	diff.Text = "C"
	hcd.recoverOneDiff(&diff)
	assert.Equal(t, "", diff.Text)
}

func TestDiffWithHighlightPlaceholder(t *testing.T) {
	hcd := newHighlightCodeDiff()
	diffs := hcd.diffWithHighlight(
		"a='\U00100000'",
		"a='\U0010FFFD''",
	)
	assert.Equal(t, "", hcd.placeholderTokenMap[0x00100000])
	assert.Equal(t, "", hcd.placeholderTokenMap[0x0010FFFD])

	expected := fmt.Sprintf(`a='<span class="removed-code">%s</span>'`, "\U00100000")
	output := diffToHTML(hcd.lineWrapperTags, diffs, DiffLineDel)
	assert.Equal(t, expected, output)

	hcd = newHighlightCodeDiff()
	diffs = hcd.diffWithHighlight(
		"a='\U00100000'",
		"a='\U0010FFFD'",
	)
	expected = fmt.Sprintf(`a='<span class="added-code">%s</span>'`, "\U0010FFFD")
	output = diffToHTML(nil, diffs, DiffLineAdd)
	assert.Equal(t, expected, output)
}

func TestDiffWithHighlightPlaceholderExhausted(t *testing.T) {
	hcd := newHighlightCodeDiff()
	hcd.placeholderMaxCount = 0
	diffs := hcd.diffWithHighlight(
		"'",
		``,
	)
	output := diffToHTML(nil, diffs, DiffLineDel)
	expected := `<span class="removed-code">'</span>`
	assert.Equal(t, expected, output)

	hcd = newHighlightCodeDiff()
	hcd.placeholderMaxCount = 0
	diffs = hcd.diffWithHighlight(
		"a this_is_not_html_at_this_point b",
		"a this_is_is_still_not_html_at_this_point_its_just_a_string b",
	)
	output = diffToHTML(nil, diffs, DiffLineDel)
	expected = "a this_is_not_html_at_this_point b"
	assert.Equal(t, expected, output)

	output = diffToHTML(nil, diffs, DiffLineAdd)
	expected = "a this_is_<span class=\"added-code\">is_still_</span>not_html_at_this_point<span class=\"added-code\">_its_just_a_string</span> b"
	assert.Equal(t, expected, output)
}
