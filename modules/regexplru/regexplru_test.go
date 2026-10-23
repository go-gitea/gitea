// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package regexplru

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegexpLru(t *testing.T) {
	cache := NewCache(2)
	r, err := cache.GetCompiled("a")
	assert.NoError(t, err)
	assert.True(t, r.MatchString("a"))

	r, err = cache.GetCompiled("a")
	assert.NoError(t, err)
	assert.True(t, r.MatchString("a"))
	assert.Equal(t, 1, cache.lruCache.Len())

	_, err = cache.GetCompiled("(")
	assert.Error(t, err)
	assert.Equal(t, 2, cache.lruCache.Len())
}
