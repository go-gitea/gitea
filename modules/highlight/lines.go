// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package highlight

import (
	"crypto/sha256"
	"encoding/binary"
	"html/template"
	"slices"
	"strings"
	"sync"

	"gitea.dev/modules/log"
	"gitea.dev/modules/util"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/hashicorp/golang-lru/v2/simplelru"
)

const (
	linesCacheEntryOverhead = 384
	linesCacheLineOverhead  = 32
)

type linesCacheKey struct {
	digest [sha256.Size]byte
	lexer  *chroma.Config
}

type linesCache struct {
	mu           sync.Mutex
	lru          *simplelru.LRU[linesCacheKey, map[int]template.HTML]
	bytes, limit int
}

var renderedLinesCache = newLinesCache(64 * 1024 * 1024)

func newLinesCache(limit int) *linesCache {
	cache := &linesCache{limit: limit}
	cache.lru, _ = simplelru.NewLRU(limit/linesCacheEntryOverhead, func(_ linesCacheKey, lines map[int]template.HTML) {
		cache.bytes -= linesCacheBytes(lines)
	}) // it only fails when size <= 0
	return cache
}

func linesCacheBytes(lines map[int]template.HTML) int {
	size := linesCacheEntryOverhead
	for _, html := range lines {
		size += linesCacheLineOverhead + len(html)
	}
	return size
}

func (cache *linesCache) get(key linesCacheKey) (map[int]template.HTML, bool) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	return cache.lru.Get(key)
}

func (cache *linesCache) add(key linesCacheKey, lines map[int]template.HTML) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.lru.Contains(key) {
		return // Add on an existing key skips the eviction callback and would count its bytes twice
	}
	cache.bytes += linesCacheBytes(lines)
	cache.lru.Add(key, lines)
	for cache.bytes > cache.limit {
		cache.lru.RemoveOldest()
	}
}

// RenderCodeLinesByLexer highlights the zero-based lines with all preceding code as lexer context, the returned map is cached and must not be modified
func RenderCodeLinesByLexer(lexer chroma.Lexer, code string, lineNumbers []int) map[int]template.HTML {
	if len(lineNumbers) == 0 {
		return nil
	}
	lineNumbers = slices.Compact(slices.Sorted(slices.Values(lineNumbers)))
	key := linesCacheKey{digest: linesDigest(code, lineNumbers), lexer: lexer.Config()}
	if lines, ok := renderedLinesCache.get(key); ok {
		return lines
	}
	lines := renderCodeLines(lexer, code, lineNumbers)
	renderedLinesCache.add(key, lines)
	return lines
}

func linesDigest(code string, lineNumbers []int) [sha256.Size]byte {
	numbers := binary.LittleEndian.AppendUint64(nil, uint64(len(code)))
	for _, line := range lineNumbers {
		numbers = binary.LittleEndian.AppendUint64(numbers, uint64(line))
	}
	hash := sha256.New()
	_, _ = hash.Write(numbers)
	_, _ = hash.Write(util.UnsafeStringToBytes(code))
	return [sha256.Size]byte(hash.Sum(nil))
}

func renderCodeLines(lexer chroma.Lexer, code string, lineNumbers []int) map[int]template.HTML {
	iterator := chroma.Literator(chroma.Token{Type: chroma.Text, Value: code})
	if lexer != lexers.Fallback {
		var err error
		if iterator, err = lexer.Tokenise(nil, code); err != nil {
			log.Error("Can't tokenize code: %v", err)
			return nil
		}
	}
	html, err := formatTokens(codeTokensForLines(iterator, lineNumbers))
	if err != nil {
		log.Error("Can't format code: %v", err)
		return nil
	}
	lines := make(map[int]template.HTML, len(lineNumbers))
	for idx, line := range UnsafeSplitHighlightedLines(template.HTML(strings.Clone(string(html)))) { // drop the buffer's spare capacity, the cache budget counts length
		lines[lineNumbers[idx]] = template.HTML(util.UnsafeBytesToString(line))
	}
	return lines
}

// codeTokensForLines yields only the tokens of the sorted lines and stops lexing after the last one
func codeTokensForLines(iterator chroma.Iterator, lineNumbers []int) chroma.Iterator {
	line, target := 0, 0
	var pending chroma.Token
	return func() chroma.Token {
		for target < len(lineNumbers) {
			if pending.Value == "" {
				if pending = iterator(); pending == chroma.EOF {
					return chroma.EOF
				}
			}
			token := pending
			nl := strings.IndexByte(token.Value, '\n')
			if nl >= 0 {
				token.Value = token.Value[:nl+1]
			}
			pending.Value = pending.Value[len(token.Value):]
			selected := line == lineNumbers[target]
			if nl >= 0 {
				line++
				if selected {
					target++
				}
			}
			if selected {
				return token
			}
		}
		return chroma.EOF
	}
}
