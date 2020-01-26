// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"

	_ "gitea.com/macaron/cache/memcache" // memcache plugin for cache
	_ "gitea.com/macaron/cache/redis"
)

// Cache represents cache settings
type Cache struct {
	Adapter  string
	Interval int
	Conn     string
	TTL      time.Duration
}

var (
	// CacheService the global cache
	CacheService *Cache
)

func newCacheService() {
	sec := Cfg.Section("cache")
	CacheService = &Cache{
		Adapter: sec.Key("ADAPTER").In("memory", []string{"memory", "redis", "memcache"}),
	}
	switch CacheService.Adapter {
	case "memory":
		CacheService.Interval = sec.Key("INTERVAL").MustInt(60)
	case "redis", "memcache":
		CacheService.Conn = strings.Trim(sec.Key("HOST").String(), "\" ")
	default:
		log.Fatal("Unknown cache adapter: %s", CacheService.Adapter)
	}
	CacheService.TTL = sec.Key("ITEM_TTL").MustDuration(16 * time.Hour)

	log.Info("Cache Service Enabled")
}
