// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"math"
	"time"

	"gitea.dev/modules/log"
)

// Cache represents cache settings
type Cache struct {
	Adapter  string
	Interval int           // GC
	Conn     string        `ini:"-"`
	TTL      time.Duration `ini:"ITEM_TTL"`
}

// CacheService the global cache
var CacheService = struct {
	Cache `ini:"cache"`

	LastCommit struct {
		TTL time.Duration `ini:"ITEM_TTL"`
	} `ini:"cache.last_commit"`
}{
	Cache: Cache{
		Adapter:  "memory",
		Interval: 60,
		TTL:      16 * time.Hour,
	},
	LastCommit: struct {
		TTL time.Duration `ini:"ITEM_TTL"`
	}{
		TTL: 8760 * time.Hour,
	},
}

func loadCacheFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("cache")
	if err := sec.MapTo(&CacheService); err != nil {
		log.Fatal("Failed to map Cache settings: %v", err)
	}

	CacheService.Adapter = sec.Key("ADAPTER").In("memory", []string{"memory", "redis", "memcache", "twoqueue"})
	switch CacheService.Adapter {
	case "memory":
	case "memcache":
		CacheService.Conn = sec.Key("HOST").String()
	case "redis":
		CacheService.Conn = sec.Key("HOST").MustString(Redis.ConnStr)
	case "twoqueue":
		CacheService.Conn = sec.Key("HOST").MustString("50000")
	default:
		log.Fatal("Unknown cache adapter: %s", CacheService.Adapter)
	}
}

// TTLSeconds returns the item TTL in seconds, negative when ITEM_TTL disables caching
func (c Cache) TTLSeconds() int64 {
	return ttlSeconds(c.TTL)
}

// LastCommitCacheTTLSeconds returns the last commit item TTL in seconds, negative when ITEM_TTL disables caching
func LastCommitCacheTTLSeconds() int64 {
	return ttlSeconds(CacheService.LastCommit.TTL)
}

func ttlSeconds(ttl time.Duration) int64 {
	if ttl < 0 {
		return -1
	}
	return int64(math.Ceil(ttl.Seconds()))
}
