// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package highlight

import (
	"html/template"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type countingHighlightLexer struct {
	chroma.Lexer
	calls atomic.Int32
}

func (lexer *countingHighlightLexer) Tokenise(options *chroma.TokeniseOptions, code string) (chroma.Iterator, error) {
	lexer.calls.Add(1)
	return lexer.Lexer.Tokenise(options, code)
}

func TestHighlightLineCacheIdentity(t *testing.T) {
	cache := newHighlightLineCache(highlightCacheBytes, highlightCacheEntryBytes)
	lexer := &countingHighlightLexer{Lexer: DetectChromaLexerByFileName("file.c", "")}
	const code = "/*\ninside <&>\n*/\nint n;\n"
	want := RenderCodeLinesByLexer(lexer.Lexer, code, []int{1, 3})
	first := cache.renderLines(lexer, code, []int{3, 1, 3, -1})
	assert.Equal(t, want, first)
	first[1] = "changed by caller"
	assert.Equal(t, want, cache.renderLines(lexer, code, []int{1, 3}))
	assert.EqualValues(t, 1, lexer.calls.Load())
	assert.Equal(t, want, cache.renderLines(DetectChromaLexerByFileName("other.c", ""), code, []int{1, 3}))
	assert.Equal(t, 1, cache.lru.Len())

	for _, test := range []struct {
		lexer chroma.Lexer
		code  string
		lines []int
	}{
		{lexer, strings.ReplaceAll(code, "inside", "changed"), []int{1, 3}},
		{lexer, code, []int{1}},
		{DetectChromaLexerByFileName("file.py", ""), code, []int{1, 3}},
	} {
		expected := RenderCodeLinesByLexer(test.lexer, test.code, test.lines)
		assert.Equal(t, expected, cache.renderLines(test.lexer, test.code, test.lines))
	}
	assert.Equal(t, 4, cache.lru.Len())
	assert.Empty(t, cache.renderLines(lexer, code, nil))
	assert.Equal(t, 4, cache.lru.Len())
}

func TestHighlightLineCacheBudget(t *testing.T) {
	const value = template.HTML("<span>hello</span>\n")
	entryBytes := highlightCacheEntryOverhead + highlightCacheLineOverhead + len(value)
	cache := newHighlightLineCache(2*entryBytes, entryBytes)
	key := func(id byte) highlightCacheKey { return highlightCacheKey{digest: [32]byte{id}} }
	render := func() map[int]template.HTML { return map[int]template.HTML{0: value} }
	cache.render(key(1), render)
	cache.render(key(2), render)
	require.NotNil(t, cache.get(key(1)))
	cache.render(key(3), render)
	assert.Nil(t, cache.get(key(2)))
	assert.NotNil(t, cache.get(key(1)))
	assert.NotNil(t, cache.get(key(3)))
	assert.Equal(t, 2*entryBytes, cache.bytes)
	assert.Equal(t, 2, cache.lru.Len())

	large := map[int]template.HTML{0: template.HTML(strings.Repeat("x", entryBytes))}
	assert.Equal(t, large, cache.render(key(4), func() map[int]template.HTML { return large }))
	assert.Nil(t, cache.get(key(4)))
	assert.Equal(t, 2, cache.lru.Len())
	assert.LessOrEqual(t, cache.bytes, cache.limit)
}

func TestHighlightLineCacheConcurrent(t *testing.T) {
	cache := newHighlightLineCache(highlightCacheBytes, highlightCacheEntryBytes)
	key := highlightCacheKey{digest: [32]byte{1}}
	var renders atomic.Int32
	var once sync.Once
	started, release := make(chan struct{}), make(chan struct{})
	render := func() map[int]template.HTML {
		renders.Add(1)
		once.Do(func() { close(started) })
		<-release
		return map[int]template.HTML{0: "hello"}
	}
	var ready, done sync.WaitGroup
	var results [32]map[int]template.HTML
	ready.Add(len(results))
	done.Add(len(results))
	for i := range results {
		go func() {
			defer done.Done()
			ready.Done()
			results[i] = cache.render(key, render)
		}()
	}
	ready.Wait()
	<-started
	close(release)
	done.Wait()
	assert.EqualValues(t, 1, renders.Load())
	results[0][0] = "changed by caller"
	for _, result := range results[1:] {
		assert.Equal(t, template.HTML("hello"), result[0])
	}
	assert.Equal(t, 1, cache.lru.Len())
}

func TestHighlightLineCacheReconfigure(t *testing.T) {
	const value = template.HTML("hello")
	entryBytes := highlightCacheEntryOverhead + highlightCacheLineOverhead + len(value)
	cache := newHighlightLineCache(2*entryBytes, highlightCacheEntryBytes)
	first := highlightCacheKey{digest: [32]byte{1}}
	second := highlightCacheKey{digest: [32]byte{2}}
	render := func() map[int]template.HTML { return map[int]template.HTML{0: value} }
	cache.render(first, render)
	cache.render(second, render)
	cache.configure(entryBytes, 2)
	assert.Nil(t, cache.get(first))
	assert.NotNil(t, cache.get(second))
	assert.Equal(t, entryBytes, cache.bytes)
	cache.configure(2*entryBytes, 1)
	assert.Equal(t, entryBytes, cache.limit)
	cache.configure(0, 3)
	assert.Zero(t, cache.lru.Len())
	assert.Zero(t, cache.bytes)
	cache.render(first, render)
	assert.Zero(t, cache.lru.Len())
	cache.configure(2*entryBytes, 4)
	cache.render(first, render)
	assert.NotNil(t, cache.get(first))
}

func TestHighlightLineCacheDisableDuringRender(t *testing.T) {
	cache := newHighlightLineCache(highlightCacheBytes, highlightCacheEntryBytes)
	key := highlightCacheKey{digest: [32]byte{1}}
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		cache.render(key, func() map[int]template.HTML {
			close(started)
			<-release
			return map[int]template.HTML{0: "hello"}
		})
	}()
	<-started
	cache.configure(0, 2)
	cache.configure(highlightCacheBytes, 3)
	close(release)
	<-done
	assert.Nil(t, cache.get(key))
	assert.Zero(t, cache.bytes)
}
