// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitdiff

import (
	"fmt"
	"html/template"
	"strings"
	"testing"

	"gitea.dev/modules/highlight"
	"gitea.dev/modules/translation"

	"github.com/alecthomas/chroma/v2"
	"github.com/stretchr/testify/assert"
)

func TestDiffWithHighlight(t *testing.T) {
	t.Run("DiffLineAddDel", func(t *testing.T) {
		t.Run("WithDiffTags", func(t *testing.T) {
			hcd := newHighlightCodeDiff()
			codeA := template.HTML(`x <span class="k">foo</span> y`)
			codeB := template.HTML(`x <span class="k">bar</span> y`)
			outDel, outAdd := hcd.diffLineWithHighlight(codeA, codeB)
			assert.Equal(t, `x <span class="removed-code"><span class="k">foo</span></span> y`, string(outDel))
			assert.Equal(t, `x <span class="added-code"><span class="k">bar</span></span> y`, string(outAdd))
		})
		t.Run("NoRedundantTags", func(t *testing.T) {
			// the equal parts only contain spaces, in this case, don't use "added/removed" tags
			// because the diff lines already have a background color to indicate the change
			hcd := newHighlightCodeDiff()
			codeA := template.HTML("<span> </span> \t<span>foo</span> ")
			codeB := template.HTML(" <span>bar</span> \n")
			outDel, outAdd := hcd.diffLineWithHighlight(codeA, codeB)
			assert.Equal(t, string(codeA), string(outDel))
			assert.Equal(t, string(codeB), string(outAdd))
		})
	})

	t.Run("CleanUp", func(t *testing.T) {
		hcd := newHighlightCodeDiff()
		codeA := template.HTML(` <span class="cm">this is a comment</span>`)
		codeB := template.HTML(` <span class="cm">this is updated comment</span>`)
		outDel, outAdd := hcd.diffLineWithHighlight(codeA, codeB)
		assert.Equal(t, ` <span class="cm">this is <span class="removed-code">a</span> comment</span>`, string(outDel))
		assert.Equal(t, ` <span class="cm">this is <span class="added-code">updated</span> comment</span>`, string(outAdd))

		codeA = `<span class="line"><span>line1</span></span>` + "\n" + `<span class="cl"><span>line2</span></span>`
		codeB = `<span class="cl"><span>line1</span></span>` + "\n" + `<span class="line"><span>line!</span></span>`
		outDel, outAdd = hcd.diffLineWithHighlight(codeA, codeB)
		assert.Equal(t, `<span>line1</span>`+"\n"+`<span class="removed-code"><span>line2</span></span>`, string(outDel))
		assert.Equal(t, `<span>line1</span>`+"\n"+`<span><span class="added-code">line!</span></span>`, string(outAdd))
	})

	t.Run("OpenCloseTags", func(t *testing.T) {
		hcd := newHighlightCodeDiff()
		hcd.placeholderTokenMap['O'], hcd.placeholderTokenMap['C'] = "<span>", "</span>"
		assert.Equal(t, "<span></span>", string(hcd.recoverOneDiff("OC")))
		assert.Equal(t, "<span></span>", string(hcd.recoverOneDiff("O")))
		assert.Empty(t, string(hcd.recoverOneDiff("C")))
	})

	t.Run("ComplexDiff1", func(t *testing.T) {
		oldCode, _, _ := highlight.RenderCodeSlowGuess("a.go", "Go", `xxx || yyy`)
		newCode, _, _ := highlight.RenderCodeSlowGuess("a.go", "Go", `bot&xxx || bot&yyy`)
		hcd := newHighlightCodeDiff()
		_, add := hcd.diffLineWithHighlight(oldCode, newCode)
		assert.Equal(t, strings.ReplaceAll(`
<span class="added-code"><span class="nx">bot</span></span><span class="o"><span class="added-code">&amp;</span></span>
<span class="nx">xxx</span><span class="w"> </span><span class="o">||</span><span class="w"> </span>
<span class="added-code"><span class="nx">bot</span></span><span class="o"><span class="added-code">&amp;</span></span>
<span class="nx">yyy</span>`, "\n", ""), string(add))
	})

	forceTokenAsPlaceholder := func(hcd *highlightCodeDiff, r rune, token string) rune {
		// for testing purpose only
		hcd.tokenPlaceholderMap[token] = r
		hcd.placeholderTokenMap[r] = token
		return r
	}

	t.Run("ComplexDiff2", func(t *testing.T) {
		// When running "diffLineWithHighlight", the newly inserted "added-code", and "removed-code" tags may break the original layout.
		// The newly inserted tags can appear in any position, because the "diff" algorithm can make outputs like:
		// * Equal: <span>
		// * Insert: xx</span><span>yy
		// * Equal: zz</span>
		// Then the newly inserted tags will make this output, the tags mismatch.
		// * <span>  <added>xx</span><span>yy</added>  zz</span>
		// So we need to fix it to:
		// * <span>  <added>xx</added></span> <span><added>yy</added>  zz</span>
		hcd := newHighlightCodeDiff()
		hcd.diffCodeAddedOpen = forceTokenAsPlaceholder(hcd, '[', "<add>")
		hcd.diffCodeClose = forceTokenAsPlaceholder(hcd, ']', "</add>")
		forceTokenAsPlaceholder(hcd, '{', "<T>")
		forceTokenAsPlaceholder(hcd, '}', "</T>")
		assert.Equal(t, `aa<T>xx<add>yy</add>zz</T>bb`, string(hcd.recoverOneDiff("aa{xx[yy]zz}bb")))
		assert.Equal(t, `aa<add>xx</add><T><add>yy</add></T><add>zz</add>bb`, string(hcd.recoverOneDiff("aa[xx{yy}zz]bb")))
		assert.Equal(t, `aa<T>xx<add>yy</add></T><add>zz</add>bb`, string(hcd.recoverOneDiff("aa{xx[yy}zz]bb")))
		assert.Equal(t, `aa<add>xx</add><T><add>yy</add>zz</T>bb`, string(hcd.recoverOneDiff("aa[xx{yy]zz}bb")))
		assert.Equal(t, `aa<add>xx</add><T><add>yy</add><add>zz</add></T><add>bb</add>cc`, string(hcd.recoverOneDiff("aa[xx{yy][zz}bb]cc")))

		// And do a simple test for "diffCodeRemovedOpen", it shares the same logic as "diffCodeAddedOpen"
		hcd = newHighlightCodeDiff()
		hcd.diffCodeRemovedOpen = forceTokenAsPlaceholder(hcd, '[', "<del>")
		hcd.diffCodeClose = forceTokenAsPlaceholder(hcd, ']', "</del>")
		forceTokenAsPlaceholder(hcd, '{', "<T>")
		forceTokenAsPlaceholder(hcd, '}', "</T>")
		assert.Equal(t, `aa<del>xx</del><T><del>yy</del><del>zz</del></T><del>bb</del>cc`, string(hcd.recoverOneDiff("aa[xx{yy][zz}bb]cc")))
	})
}

func TestDiffWithHighlightPlaceholder(t *testing.T) {
	hcd := newHighlightCodeDiff()
	output, _ := hcd.diffLineWithHighlight("a='\U00100000'", "a='\U0010FFFD''")
	assert.Empty(t, hcd.placeholderTokenMap[0x00100000])
	assert.Empty(t, hcd.placeholderTokenMap[0x0010FFFD])
	expected := fmt.Sprintf(`a='<span class="removed-code">%s</span>'`, "\U00100000")
	assert.Equal(t, expected, string(output))

	hcd = newHighlightCodeDiff()
	_, output = hcd.diffLineWithHighlight("a='\U00100000'", "a='\U0010FFFD'")
	expected = fmt.Sprintf(`a='<span class="added-code">%s</span>'`, "\U0010FFFD")
	assert.Equal(t, expected, string(output))
}

func TestDiffWithHighlightPlaceholderExhausted(t *testing.T) {
	hcd := newHighlightCodeDiff()
	hcd.placeholderMaxCount = 0
	placeHolderAmp := string(rune(0xFFFD))
	del, add := hcd.diffLineWithHighlight(`<span class="k">&lt;</span>`, `<span class="k">&gt;</span>`)
	assert.Equal(t, placeHolderAmp+"lt;", string(del))
	assert.Equal(t, placeHolderAmp+"gt;", string(add))

	del, add = hcd.diffLineWithHighlight(`<span class="k">foo</span>`, `<span class="k">bar</span>`)
	assert.Equal(t, "foo", string(del))
	assert.Equal(t, "bar", string(add))
}

func TestDiffWithHighlightTagMatch(t *testing.T) {
	totalOverflow := 0
	for i := 0; ; i++ {
		hcd := newHighlightCodeDiff()
		hcd.placeholderMaxCount = i
		del, add := hcd.diffLineWithHighlight(`<span class="k">&lt;</span>`, `<span class="k">&gt;</span>`)
		totalOverflow += hcd.placeholderOverflowCount
		assert.Equal(t, strings.Count(string(del), "<span"), strings.Count(string(del), "</span"))
		assert.Equal(t, strings.Count(string(add), "<span"), strings.Count(string(add), "</span"))
		if hcd.placeholderOverflowCount == 0 {
			break
		}
	}
	assert.NotZero(t, totalOverflow)
}

func BenchmarkHighlightDiff(b *testing.B) {
	// still fast enough: BenchmarkHighlightDiff-12    	 1000000	      1027 ns/op
	// HINT: CODE-HIGHLIGHT-PERFORMANCE: the real bottleneck is in the Chroma highlighter.
	for b.Loop() {
		hcd := newHighlightCodeDiff()
		codeA := template.HTML(`x <span class="k">foo</span> y`)
		codeB := template.HTML(`x <span class="k">bar</span> y`)
		hcd.diffLineWithHighlight(codeA, codeB)
	}
}

func BenchmarkGetDiffLineForRender(b *testing.B) {
	diffSection := &DiffSection{
		FileName:       "test.go",
		highlightLexer: &diffVarMutable[chroma.Lexer]{},
	}
	leftLine := &DiffLine{LeftIdx: 1, Content: `-x <span class="k">foo</span> y`}
	rightLine := &DiffLine{RightIdx: 1, Content: `+x <span class="k">bar</span> y`}
	locale := translation.MockLocale{}

	b.ResetTimer()
	// HINT: CODE-HIGHLIGHT-PERFORMANCE: the real bottleneck is in the Chroma highlighter.
	for b.Loop() {
		// Clear cache only at the start of rendering the pair
		leftLine.cachedDiffInline = nil
		rightLine.cachedDiffInline = nil
		_ = diffSection.getDiffLineForRender(DiffLineDel, leftLine, rightLine, locale)
		_ = diffSection.getDiffLineForRender(DiffLineAdd, leftLine, rightLine, locale)
	}
}
