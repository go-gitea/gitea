// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitdiff

import (
	"fmt"
	"html/template"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiffWithHighlight(t *testing.T) {
	t.Run("DiffLineAddDel", func(t *testing.T) {
		hcd := newHighlightCodeDiff()
		codeA := template.HTML(`x <span class="k">foo</span> y`)
		codeB := template.HTML(`x <span class="k">bar</span> y`)
		outDel := hcd.diffLineWithHighlight(DiffLineDel, codeA, codeB)
		assert.Equal(t, `x <span class="k"><span class="removed-code">foo</span></span> y`, string(outDel))
		outAdd := hcd.diffLineWithHighlight(DiffLineAdd, codeA, codeB)
		assert.Equal(t, `x <span class="k"><span class="added-code">bar</span></span> y`, string(outAdd))
	})

	t.Run("CleanUp", func(t *testing.T) {
		hcd := newHighlightCodeDiff()
		codeA := template.HTML(`<span class="cm>this is a comment</span>`)
		codeB := template.HTML(`<span class="cm>this is updated comment</span>`)
		outDel := hcd.diffLineWithHighlight(DiffLineDel, codeA, codeB)
		assert.Equal(t, `<span class="cm>this is <span class="removed-code">a</span> comment</span>`, string(outDel))
		outAdd := hcd.diffLineWithHighlight(DiffLineAdd, codeA, codeB)
		assert.Equal(t, `<span class="cm>this is <span class="added-code">updated</span> comment</span>`, string(outAdd))
	})

	t.Run("OpenCloseTags", func(t *testing.T) {
		hcd := newHighlightCodeDiff()
		hcd.placeholderTokenMap['O'], hcd.placeholderTokenMap['C'] = "<span>", "</span>"
		assert.Equal(t, "<span></span>", string(hcd.recoverOneDiff("OC")))
		assert.Equal(t, "<span></span>", string(hcd.recoverOneDiff("O")))
		assert.Equal(t, "", string(hcd.recoverOneDiff("C")))
	})
}

func TestDiffWithHighlightPlaceholder(t *testing.T) {
	hcd := newHighlightCodeDiff()
	output := hcd.diffLineWithHighlight(DiffLineDel, "a='\U00100000'", "a='\U0010FFFD''")
	assert.Equal(t, "", hcd.placeholderTokenMap[0x00100000])
	assert.Equal(t, "", hcd.placeholderTokenMap[0x0010FFFD])
	expected := fmt.Sprintf(`a='<span class="removed-code">%s</span>'`, "\U00100000")
	assert.Equal(t, expected, string(output))

	hcd = newHighlightCodeDiff()
	output = hcd.diffLineWithHighlight(DiffLineAdd, "a='\U00100000'", "a='\U0010FFFD'")
	expected = fmt.Sprintf(`a='<span class="added-code">%s</span>'`, "\U0010FFFD")
	assert.Equal(t, expected, string(output))
}

func TestDiffWithHighlightPlaceholderExhausted(t *testing.T) {
	hcd := newHighlightCodeDiff()
	hcd.placeholderMaxCount = 0
	placeHolderAmp := string(rune(0xFFFD))
	output := hcd.diffLineWithHighlight(DiffLineDel, `<span class="k">&lt;</span>`, `<span class="k">&gt;</span>`)
	assert.Equal(t, placeHolderAmp+"lt;", string(output))
	output = hcd.diffLineWithHighlight(DiffLineAdd, `<span class="k">&lt;</span>`, `<span class="k">&gt;</span>`)
	assert.Equal(t, placeHolderAmp+"gt;", string(output))
}

func TestDiffWithHighlightTagMatch(t *testing.T) {
	f := func(t *testing.T, lineType DiffLineType) {
		totalOverflow := 0
		for i := 0; ; i++ {
			hcd := newHighlightCodeDiff()
			hcd.placeholderMaxCount = i
			output := string(hcd.diffLineWithHighlight(lineType, `<span class="k">&lt;</span>`, `<span class="k">&gt;</span>`))
			totalOverflow += hcd.placeholderOverflowCount
			assert.Equal(t, strings.Count(output, "<span"), strings.Count(output, "</span"))
			if hcd.placeholderOverflowCount == 0 {
				break
			}
		}
		assert.NotZero(t, totalOverflow)
	}
	t.Run("DiffLineAdd", func(t *testing.T) { f(t, DiffLineAdd) })
	t.Run("DiffLineDel", func(t *testing.T) { f(t, DiffLineDel) })
}
