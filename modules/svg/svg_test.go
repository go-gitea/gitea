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
	svgRenderedHTMLCache.Clear()

	// default params: no cache entry
	RenderHTML("test")
	_, ok := svgRenderedHTMLCache.Load(svgCacheKey{"test", 16, ""})
	assert.False(t, ok)

	// non-default params: cached
	RenderHTML("test", 24)
	_, ok = svgRenderedHTMLCache.Load(svgCacheKey{"test", 24, ""})
	assert.True(t, ok)
}
