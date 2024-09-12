// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package regexplru

import (
	"regexp"

	"code.gitea.io/gitea/modules/log"

	lru "github.com/hashicorp/golang-lru/v2"
)

var lruCache *lru.Cache[string, any]

func init() {
	var err error
	lruCache, err = lru.New[string, any](1000)
	if err != nil {
		log.Fatal("failed to new LRU cache, err: %v", err)
	}
}

// GetCompiled works like regexp.Compile, the compiled expr or error is stored in LRU cache
func GetCompiled(expr string) (r *regexp.Regexp, err error) {
	v, ok := lruCache.Get(expr)
	if !ok {
		r, err = regexp.Compile(expr)
		if err != nil {
			lruCache.Add(expr, err)
			return nil, err
		}
		lruCache.Add(expr, r)
	} else {
		r, ok = v.(*regexp.Regexp)
		if !ok {
			if err, ok = v.(error); ok {
				return nil, err
			}
			panic("impossible")
		}
	}
	return r, nil
}
