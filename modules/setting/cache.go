// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
)

// Cache represents cache settings
type Cache struct {
	Adapter  string
	Interval int
	Conn     string
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

// MemcacheMaxTTL represents the maximum memcache TTL
const MemcacheMaxTTL = 30 * 24 * time.Hour

func loadCacheFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("cache")
	if err := sec.MapTo(&CacheService); err != nil {
		log.Fatal("Failed to map Cache settings: %v", err)
	}

	CacheService.Adapter = sec.Key("ADAPTER").In("memory", []string{"memory", "redis", "memcache", "twoqueue"})
	switch CacheService.Adapter {
	case "memory":
	case "redis", "memcache":
		CacheService.Conn = strings.Trim(sec.Key("HOST").String(), "\" ")
	case "twoqueue":
		CacheService.Conn = strings.TrimSpace(sec.Key("HOST").String())
		if CacheService.Conn == "" {
			CacheService.Conn = "50000"
		}
	default:
		log.Fatal("Unknown cache adapter: %s", CacheService.Adapter)
	}
}

// TTLSeconds returns the TTLSeconds or unix timestamp for memcache
func (c Cache) TTLSeconds() int64 {
	if c.Adapter == "memcache" && c.TTL > MemcacheMaxTTL {
		return time.Now().Add(c.TTL).Unix()
	}
	return int64(c.TTL.Seconds())
}

// LastCommitCacheTTLSeconds returns the TTLSeconds or unix timestamp for memcache
func LastCommitCacheTTLSeconds() int64 {
	if CacheService.Adapter == "memcache" && CacheService.LastCommit.TTL > MemcacheMaxTTL {
		return time.Now().Add(CacheService.LastCommit.TTL).Unix()
	}
	return int64(CacheService.LastCommit.TTL.Seconds())
}
