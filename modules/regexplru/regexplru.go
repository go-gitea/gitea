// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package regexplru

import (
	"regexp"
	"sync"

	lru "github.com/hashicorp/golang-lru/v2"
)

type lruItem struct {
	regexp *regexp.Regexp
	err    error
}

type RegexpCache struct {
	lruCache *lru.Cache[string, *lruItem]
}

func NewCache(size int) *RegexpCache {
	lruCache, _ := lru.New[string, *lruItem](size)
	return &RegexpCache{lruCache: lruCache}
}

// GetCompiled works like regexp.Compile, the compiled expr or error is stored in LRU cache
func (regexpCache *RegexpCache) GetCompiled(expr string) (r *regexp.Regexp, err error) {
	v, ok := regexpCache.lruCache.Get(expr)
	if !ok {
		r, err = regexp.Compile(expr)
		regexpCache.lruCache.Add(expr, &lruItem{regexp: r, err: err})
		return r, err
	}
	return v.regexp, v.err
}

var (
	UserCache   = sync.OnceValue(func() *RegexpCache { return NewCache(1000) })
	SystemCache = sync.OnceValue(func() *RegexpCache { return NewCache(1000) })
)
