// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package highlight

import (
	"crypto/sha256"
	"fmt"
	"html/template"
	"sync"
	"testing"

	"gitea.dev/modules/test"

	"github.com/alecthomas/chroma/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type countingLexer struct {
	chroma.Lexer
	calls int
}

func (lexer *countingLexer) Tokenise(options *chroma.TokeniseOptions, code string) (chroma.Iterator, error) {
	lexer.calls++
	return lexer.Lexer.Tokenise(options, code)
}

func TestRenderCodeLinesByLexer(t *testing.T) {
	for _, testCase := range []struct {
		name, code string
	}{
		{"test.c", "/* before\ninside <&>\nafter */\nint n = 1;\n"},
		{"test.py", "s = '''before\ninside λ\nafter'''\nprint(s)"},
		{"test.sh", "cat <<EOF\nhello $USER\nEOF\necho done\n"},
		{"test.html", "<script>\n/* before\ninside\nafter */\n</script>\n"},
		{"test.sql", "-- before\nSELECT 'hello\nworld';\n\nSELECT 2;\n"},
		{"test.txt", "<>&\n\nλ\rvalue\nend"},
		{"empty.py", ""},
		{"empty.txt", ""},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			lexer := DetectChromaLexerByFileName(testCase.name, "")
			full := UnsafeSplitHighlightedLines(RenderCodeByLexer(lexer, testCase.code))
			for _, requested := range [][]int{{0}, {1}, {2, 0, 2, 99}, {0, 1, 2, 3, 4, 5, 6}} {
				want := map[int]template.HTML{}
				for _, line := range requested {
					if line < len(full) {
						want[line] = template.HTML(full[line])
					}
				}
				assert.Equal(t, want, RenderCodeLinesByLexer(lexer, testCase.code, requested), "lines=%v", requested)
			}
		})
	}
}

func TestRenderCodeLinesByLexerCache(t *testing.T) {
	defer test.MockVariableValue(&renderedLinesCache, newLinesCache(1024*1024))()
	lexer := &countingLexer{Lexer: DetectChromaLexerByFileName("a.c", "")}
	const code = "/*\ninside\n*/\nint n;\n"
	lines := RenderCodeLinesByLexer(lexer, code, []int{3, 1})
	assert.Equal(t, lines, RenderCodeLinesByLexer(lexer, code, []int{1, 3}))
	assert.Equal(t, 1, lexer.calls)
	RenderCodeLinesByLexer(lexer, code, []int{1})
	RenderCodeLinesByLexer(lexer, code+"\n", []int{1, 3})
	assert.Equal(t, 3, lexer.calls)
}

func TestRenderCodeLinesByLexerConcurrentEvictions(t *testing.T) {
	defer test.MockVariableValue(&renderedLinesCache, newLinesCache(2*linesCacheEntryOverhead))()
	lexer := DetectChromaLexerByFileName("a.c", "")
	var wg sync.WaitGroup
	for idx := range 64 {
		wg.Go(func() {
			lines := RenderCodeLinesByLexer(lexer, fmt.Sprintf("/*\n%d\n*/\n", idx%8), []int{1})
			assert.Equal(t, template.HTML(fmt.Sprintf("<span class=\"cm\">%d\n</span>", idx%8)), lines[1])
		})
	}
	wg.Wait()
	assert.LessOrEqual(t, renderedLinesCache.bytes, renderedLinesCache.limit)
}

func TestLinesCacheBudget(t *testing.T) {
	lines := map[int]template.HTML{0: "x"}
	cache := newLinesCache(2 * linesCacheBytes(lines))
	for _, id := range []byte{0, 1, 2, 2} {
		cache.add(linesCacheKey{digest: [sha256.Size]byte{id}}, lines)
	}
	_, ok := cache.get(linesCacheKey{digest: [sha256.Size]byte{0}})
	assert.False(t, ok)
	assert.Equal(t, 2, cache.lru.Len())
	assert.Equal(t, 2*linesCacheBytes(lines), cache.bytes)
}

func TestCodeTokensForLinesStopsAfterLastLine(t *testing.T) {
	calls := 0
	iterator := func() chroma.Token {
		calls++
		require.Equal(t, 1, calls)
		return chroma.Token{Type: chroma.CommentMultiline, Value: "/* before\ninside\nafter */\nunused\n"}
	}
	assert.Equal(t, []chroma.Token{
		{Type: chroma.CommentMultiline, Value: "inside\n"},
		{Type: chroma.CommentMultiline, Value: "after */\n"},
	}, codeTokensForLines(iterator, []int{1, 2}).Tokens())
}
