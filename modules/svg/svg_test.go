// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package svg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderHTMLCache(t *testing.T) {
	svgIcons = map[string]string{
		"test": `<svg class="svg test" width="16" height="16"></svg>`,
	}
	clearSVGRenderCache()

	// default params: no cache entry
	RenderHTML("test")
	_, ok := svgRenderedHTMLCache.Load(svgCacheKey{"test", 16, ""})
	assert.False(t, ok)

	// non-default params: cached
	RenderHTML("test", 24)
	_, ok = svgRenderedHTMLCache.Load(svgCacheKey{"test", 24, ""})
	assert.True(t, ok)
}

func TestMockIconClearsCache(t *testing.T) {
	svgIcons = map[string]string{
		"test": `<svg class="svg test" width="16" height="16"></svg>`,
	}
	clearSVGRenderCache()

	RenderHTML("test", 24)
	restore := MockIcon("test")
	_, ok := svgRenderedHTMLCache.Load(svgCacheKey{"test", 24, ""})
	assert.False(t, ok, "MockIcon should clear cache")

	RenderHTML("test", 24)
	restore()
	_, ok = svgRenderedHTMLCache.Load(svgCacheKey{"test", 24, ""})
	assert.False(t, ok, "restore should clear cache")
}
