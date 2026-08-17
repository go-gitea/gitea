// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"sync"

	"gitea.dev/modules/setting"

	lru "github.com/hashicorp/golang-lru/v2"
)

type TokenCacheItem struct {
	TokenID   int64
	TokenHash string
}

var TokenCache = sync.OnceValue(func() *lru.Cache[string, *TokenCacheItem] {
	cacheSize := max(setting.SuccessfulTokensCacheSize, 20)
	c, _ := lru.New[string, *TokenCacheItem](cacheSize) // it only fails when size <= 0
	return c
})
