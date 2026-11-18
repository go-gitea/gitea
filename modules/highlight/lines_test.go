// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package highlight

import (
	"crypto/sha256"
	"fmt"
	"html/template"
	"strings"
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
	for name, code := range map[string]string{
		"a.c":   "/* before\ninside <&>\nafter */\nint n = 1;\n",
		"a.py":  "s = '''before\ninside λ\nafter'''\nprint(s)",
		"a.txt": "<>&\n\nλ\rvalue\nend",
	} {
		lexer := DetectChromaLexerByFileName(name, "")
		full := UnsafeSplitHighlightedLines(RenderCodeByLexer(lexer, code))
		want := map[int]template.HTML{0: template.HTML(full[0]), 1: template.HTML(full[1]), 3: template.HTML(full[3])}
		assert.Equal(t, want, RenderCodeLinesByLexer(lexer, code, []int{3, 1, 0, 1, 99}), name)
	}
}

func TestRenderCodeLinesByLexerCache(t *testing.T) {
	defer test.MockVariableValue(&renderedLinesCache, newLinesCache(1024*1024))()
	lexer := &countingLexer{Lexer: DetectChromaLexerByFileName("a.c", "")}
	RenderCodeLinesByLexer(lexer, "a\nb\n", []int{1, 0})
	RenderCodeLinesByLexer(lexer, "a\nb\n", []int{0, 1})
	assert.Equal(t, 1, lexer.calls)
	RenderCodeLinesByLexer(lexer, "a\nb\n", []int{1})
	RenderCodeLinesByLexer(lexer, "a\nc\n", []int{1})
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
}

func TestLinesCacheBudget(t *testing.T) {
	lines := map[int]template.HTML{0: template.HTML(strings.Repeat("x", 400))}
	cache := newLinesCache(linesCacheBytes(lines))
	for _, id := range []byte{0, 0, 1} {
		cache.add(linesCacheKey{digest: [sha256.Size]byte{id}}, lines)
	}
	assert.Equal(t, []linesCacheKey{{digest: [sha256.Size]byte{1}}}, cache.lru.Keys())
	assert.Equal(t, linesCacheBytes(lines), cache.bytes)
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
