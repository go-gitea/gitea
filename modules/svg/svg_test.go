// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package svg

import (
	"testing"

	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestRenderHTMLCache(t *testing.T) {
	const svgRealContent = "RealContent"
	svgIcons = map[string]svgIconItem{
		"test": {html: `<svg class="svg test" width="16" height="16">` + svgRealContent + `</svg>`},
	}

	// default params: no cache entry
	_, usingCache := renderHTML("test")
	assert.False(t, usingCache)
	_, usingCache = renderHTML("test")
	assert.False(t, usingCache)

	// non-default params: cached
	_, usingCache = renderHTML("test", 24)
	assert.False(t, usingCache)
	_, usingCache = renderHTML("test", 24)
	assert.True(t, usingCache)

	// mocked svg shouldn't be cached
	revertMock := MockIcon("test")
	mockedHTML, usingCache := renderHTML("test", 24)
	assert.False(t, usingCache)
	assert.NotContains(t, mockedHTML, svgRealContent)
	revertMock()
	realHTML, usingCache := renderHTML("test", 24)
	assert.True(t, usingCache)
	assert.Contains(t, realHTML, svgRealContent)

	t.Run("CacheWithLimit", func(t *testing.T) {
		assert.NotZero(t, svgCacheCount)
		const testLimit = 3
		defer test.MockVariableValue(&svgCacheLimit, testLimit)()
		for i := range 10 {
			_, usingCache = renderHTML("test", 100+i)
			assert.False(t, usingCache)
			_, usingCache = renderHTML("test", 100+i)
			assert.True(t, usingCache)
			assert.LessOrEqual(t, svgCacheCount, testLimit)
		}
	})
}
