// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitdiff

import (
	"fmt"
	"html/template"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/highlight"

	"github.com/stretchr/testify/assert"
)

func BenchmarkHighlightDiff(b *testing.B) {
	for b.Loop() {
		// still fast enough: BenchmarkHighlightDiff-12    	 1000000	      1027 ns/op
		// TODO: the real bottleneck is that "diffLineWithHighlight" is called twice when rendering "added" and "removed" lines by the caller
		// Ideally the caller should cache the diff result, and then use the diff result to render "added" and "removed" lines separately
		hcd := newHighlightCodeDiff()
		codeA := template.HTML(`x <span class="k">foo</span> y`)
		codeB := template.HTML(`x <span class="k">bar</span> y`)
		hcd.diffLineWithHighlight(DiffLineDel, codeA, codeB)
	}
}

func TestDiffWithHighlight(t *testing.T) {
	t.Run("DiffLineAddDel", func(t *testing.T) {
		t.Run("WithDiffTags", func(t *testing.T) {
			hcd := newHighlightCodeDiff()
			codeA := template.HTML(`x <span class="k">foo</span> y`)
			codeB := template.HTML(`x <span class="k">bar</span> y`)
			outDel := hcd.diffLineWithHighlight(DiffLineDel, codeA, codeB)
			assert.Equal(t, `x <span class="removed-code"><span class="k">foo</span></span> y`, string(outDel))
			outAdd := hcd.diffLineWithHighlight(DiffLineAdd, codeA, codeB)
			assert.Equal(t, `x <span class="added-code"><span class="k">bar</span></span> y`, string(outAdd))
		})
		t.Run("NoRedundantTags", func(t *testing.T) {
			// the equal parts only contain spaces, in this case, don't use "added/removed" tags
			// because the diff lines already have a background color to indicate the change
			hcd := newHighlightCodeDiff()
			codeA := template.HTML("<span> </span> \t<span>foo</span> ")
			codeB := template.HTML(" <span>bar</span> \n")
			outDel := hcd.diffLineWithHighlight(DiffLineDel, codeA, codeB)
			assert.Equal(t, string(codeA), string(outDel))
			outAdd := hcd.diffLineWithHighlight(DiffLineAdd, codeA, codeB)
			assert.Equal(t, string(codeB), string(outAdd))
		})
	})

	t.Run("CleanUp", func(t *testing.T) {
		hcd := newHighlightCodeDiff()
		codeA := template.HTML(` <span class="cm">this is a comment</span>`)
		codeB := template.HTML(` <span class="cm">this is updated comment</span>`)
		outDel := hcd.diffLineWithHighlight(DiffLineDel, codeA, codeB)
		assert.Equal(t, ` <span class="cm">this is <span class="removed-code">a</span> comment</span>`, string(outDel))
		outAdd := hcd.diffLineWithHighlight(DiffLineAdd, codeA, codeB)
		assert.Equal(t, ` <span class="cm">this is <span class="added-code">updated</span> comment</span>`, string(outAdd))

		codeA = `<span class="line"><span>line1</span></span>` + "\n" + `<span class="cl"><span>line2</span></span>`
		codeB = `<span class="cl"><span>line1</span></span>` + "\n" + `<span class="line"><span>line!</span></span>`
		outDel = hcd.diffLineWithHighlight(DiffLineDel, codeA, codeB)
		assert.Equal(t, `<span>line1</span>`+"\n"+`<span class="removed-code"><span>line2</span></span>`, string(outDel))
		outAdd = hcd.diffLineWithHighlight(DiffLineAdd, codeA, codeB)
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
		out := hcd.diffLineWithHighlight(DiffLineAdd, oldCode, newCode)
		assert.Equal(t, strings.ReplaceAll(`
<span class="added-code"><span class="nx">bot</span></span><span class="o"><span class="added-code">&amp;</span></span>
<span class="nx">xxx</span><span class="w"> </span><span class="o">||</span><span class="w"> </span>
<span class="added-code"><span class="nx">bot</span></span><span class="o"><span class="added-code">&amp;</span></span>
<span class="nx">yyy</span>`, "\n", ""), string(out))
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
	output := hcd.diffLineWithHighlight(DiffLineDel, "a='\U00100000'", "a='\U0010FFFD''")
	assert.Empty(t, hcd.placeholderTokenMap[0x00100000])
	assert.Empty(t, hcd.placeholderTokenMap[0x0010FFFD])
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

	output = hcd.diffLineWithHighlight(DiffLineDel, `<span class="k">foo</span>`, `<span class="k">bar</span>`)
	assert.Equal(t, "foo", string(output))
	output = hcd.diffLineWithHighlight(DiffLineAdd, `<span class="k">foo</span>`, `<span class="k">bar</span>`)
	assert.Equal(t, "bar", string(output))
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
