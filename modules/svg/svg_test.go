// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package svg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderHTMLCache(t *testing.T) {
	svgIcons = map[string]string{
		"test-icon": `<svg class="svg test-icon" width="16" height="16"><path/></svg>`,
	}
	svgRenderedHTMLCache.Clear()

	// default size and no class: fast path, no cache entry
	result := RenderHTML("test-icon")
	assert.Equal(t, `<svg class="svg test-icon" width="16" height="16"><path/></svg>`, string(result))
	_, cached := svgRenderedHTMLCache.Load(svgCacheKey{"test-icon", 16, ""})
	assert.False(t, cached, "default params should not create a cache entry")

	// custom size: should cache
	result = RenderHTML("test-icon", 24)
	assert.Contains(t, string(result), `width="24"`)
	assert.Contains(t, string(result), `height="24"`)
	_, cached = svgRenderedHTMLCache.Load(svgCacheKey{"test-icon", 24, ""})
	assert.True(t, cached, "custom size should create a cache entry")

	// custom class: should cache
	result = RenderHTML("test-icon", 16, "extra")
	assert.Contains(t, string(result), `class="extra svg test-icon"`)
	_, cached = svgRenderedHTMLCache.Load(svgCacheKey{"test-icon", 16, "extra"})
	assert.True(t, cached, "custom class should create a cache entry")

	// cache hit returns same result
	result2 := RenderHTML("test-icon", 24)
	assert.Equal(t, result2, RenderHTML("test-icon", 24))

	// missing icon returns dummy span
	result = RenderHTML("nonexistent", 16, "cls")
	assert.Contains(t, string(result), "<span>")
	assert.Contains(t, string(result), "nonexistent")
}

func TestMockIconClearsCache(t *testing.T) {
	svgIcons = map[string]string{
		"mock-icon": `<svg class="svg mock-icon" width="16" height="16"><path/></svg>`,
	}
	svgRenderedHTMLCache.Clear()

	// populate cache
	RenderHTML("mock-icon", 24)
	_, cached := svgRenderedHTMLCache.Load(svgCacheKey{"mock-icon", 24, ""})
	assert.True(t, cached)

	// MockIcon should clear cache
	restore := MockIcon("mock-icon")
	_, cached = svgRenderedHTMLCache.Load(svgCacheKey{"mock-icon", 24, ""})
	assert.False(t, cached, "MockIcon should clear cache")

	// restore should also clear cache
	RenderHTML("mock-icon", 24)
	restore()
	_, cached = svgRenderedHTMLCache.Load(svgCacheKey{"mock-icon", 24, ""})
	assert.False(t, cached, "restore should clear cache")
}
