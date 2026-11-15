// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package highlight

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"html/template"
	"strings"
	"sync"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"

	"github.com/alecthomas/chroma/v2"
	"github.com/hashicorp/golang-lru/v2/simplelru"
	"golang.org/x/sync/singleflight"
)

const (
	highlightCacheBytes      = setting.DefaultDiffHighlightCacheSizeMiB * 1024 * 1024
	highlightCacheEntryBytes = 1024 * 1024
	// Include space for keys, LRU nodes, slices, map slots, and allocator rounding.
	highlightCacheEntryOverhead = 384
	highlightCacheLineOverhead  = 32
)

type highlightCacheKey struct {
	digest [sha256.Size]byte
	lexer  *chroma.Config
}

type highlightCacheLine struct {
	number int
	html   template.HTML
}

type highlightCacheEntry struct {
	key   highlightCacheKey
	lines []highlightCacheLine
	bytes int
}

type highlightLineCache struct {
	mu                     sync.Mutex
	lru                    *simplelru.LRU[highlightCacheKey, *highlightCacheEntry]
	bytes, limit, maxEntry int
	revision               int
	generation             uint64
	flight                 singleflight.Group
}

var diffHighlightCache = newHighlightLineCache(highlightCacheBytes, highlightCacheEntryBytes)

func newHighlightLineCache(limit, maxEntry int) *highlightLineCache {
	cache := &highlightLineCache{limit: limit, maxEntry: maxEntry}
	var err error
	cache.lru, err = simplelru.NewLRU(max(1, limit/highlightCacheEntryOverhead), func(_ highlightCacheKey, entry *highlightCacheEntry) {
		cache.bytes -= entry.bytes
	})
	util.MustNoError(err)
	return cache
}

// RenderCodeLinesByLexerCached reuses highlighting for identical content, lexer configuration, and line selection.
func RenderCodeLinesByLexerCached(ctx context.Context, lexer chroma.Lexer, code string, lineNumbers []int) map[int]template.HTML {
	UpdateDiffHighlightCache(ctx)
	return diffHighlightCache.renderLines(lexer, code, lineNumbers)
}

// UpdateDiffHighlightCache applies saved limits and clears entries when caching is disabled.
func UpdateDiffHighlightCache(ctx context.Context) {
	settings, revision, _ := setting.Config().Repository.DiffHighlightCache.ValueRevision(ctx)
	limit := 0
	if settings.Enabled && settings.Valid() {
		limit = settings.SizeMiB * 1024 * 1024
	}
	diffHighlightCache.configure(limit, revision)
}

func (cache *highlightLineCache) configure(limit, revision int) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if revision < cache.revision {
		return
	}
	cache.revision = revision
	if limit == cache.limit {
		return
	}
	cache.limit = limit
	cache.generation++
	cache.lru.Resize(max(1, limit/highlightCacheEntryOverhead))
	for cache.bytes > limit {
		cache.lru.RemoveOldest()
	}
}

func (cache *highlightLineCache) renderLines(lexer chroma.Lexer, code string, lineNumbers []int) map[int]template.HTML {
	lineNumbers = normalizeLineNumbers(lineNumbers)
	if len(lineNumbers) == 0 {
		return map[int]template.HTML{}
	}
	key := makeHighlightCacheKey(lexer, code, lineNumbers)
	return cache.render(key, func() map[int]template.HTML {
		return renderCodeLinesByLexer(lexer, code, lineNumbers)
	})
}

func makeHighlightCacheKey(lexer chroma.Lexer, code string, lineNumbers []int) highlightCacheKey {
	hash := sha256.New()
	var number [8]byte
	binary.LittleEndian.PutUint64(number[:], uint64(len(code)))
	_, _ = hash.Write(number[:])
	_, _ = hash.Write(util.UnsafeStringToBytes(code))
	for _, line := range lineNumbers {
		binary.LittleEndian.PutUint64(number[:], uint64(line))
		_, _ = hash.Write(number[:])
	}
	return highlightCacheKey{digest: [sha256.Size]byte(hash.Sum(nil)), lexer: lexer.Config()}
}

func (cache *highlightLineCache) render(key highlightCacheKey, render func() map[int]template.HTML) map[int]template.HTML {
	cache.mu.Lock()
	limit, generation := cache.limit, cache.generation
	entry, _ := cache.lru.Get(key)
	cache.mu.Unlock()
	if limit == 0 {
		return render()
	}
	if entry != nil {
		return entry.result()
	}
	value, _, _ := cache.flight.Do(fmt.Sprintf("%x:%p:%d", key.digest, key.lexer, generation), func() (any, error) {
		if entry := cache.get(key); entry != nil {
			return entry, nil
		}
		result := render()
		entry := &highlightCacheEntry{key: key, bytes: highlightCacheEntryOverhead + len(result)*highlightCacheLineOverhead}
		for _, html := range result {
			entry.bytes += len(html)
		}
		store := entry.bytes <= cache.maxEntry && entry.bytes <= limit
		entry.lines = make([]highlightCacheLine, 0, len(result))
		for number, html := range result {
			if store {
				html = template.HTML(strings.Clone(string(html)))
			}
			entry.lines = append(entry.lines, highlightCacheLine{number: number, html: html})
		}
		if store {
			cache.put(entry, generation)
		}
		return entry, nil
	})
	entry, _ = value.(*highlightCacheEntry)
	return entry.result()
}

func (entry *highlightCacheEntry) result() map[int]template.HTML {
	result := make(map[int]template.HTML, len(entry.lines))
	for _, line := range entry.lines {
		result[line.number] = line.html
	}
	return result
}

func (cache *highlightLineCache) get(key highlightCacheKey) *highlightCacheEntry {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	entry, _ := cache.lru.Get(key)
	return entry
}

func (cache *highlightLineCache) put(entry *highlightCacheEntry, generation uint64) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if generation != cache.generation || cache.limit == 0 || entry.bytes > cache.limit {
		return
	}
	cache.bytes += entry.bytes
	cache.lru.Add(entry.key, entry)
	for cache.bytes > cache.limit {
		cache.lru.RemoveOldest()
	}
}
