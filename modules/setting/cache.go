// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
)

var (
	// CacheService represents cache settings
	CacheService = struct {
		Adapter  string
		Interval int
		Conn     string
		TTL      time.Duration

		LastCommit struct {
			Type                 string
			EnableMinCommitCount int64
			ConnStr              string
		} `ini:"cache.last_commit"`
		LsTree struct {
			Type                 string
			EnableMinCommitCount int64
			ConnStr              string
		} `ini:"cache.ls_tree"`
	}{
		LastCommit: struct {
			Type                 string
			EnableMinCommitCount int64
			ConnStr              string
		}{
			Type:                 "",
			EnableMinCommitCount: 0,
			ConnStr:              "",
		},
		LsTree: struct {
			Type                 string
			EnableMinCommitCount int64
			ConnStr              string
		}{
			Type:                 "",
			EnableMinCommitCount: 0,
			ConnStr:              "",
		},
	}
)

func newCacheService() {
	sec := Cfg.Section("cache")
	if err := sec.MapTo(&CacheService); err != nil {
		log.Fatal(4, "Failed to map Cache settings: %v", err)
	}

	CacheService.Adapter = sec.Key("ADAPTER").In("memory", []string{"memory", "redis", "memcache"})

	switch CacheService.Adapter {
	case "memory":
		CacheService.Interval = sec.Key("INTERVAL").MustInt(60)
	case "redis", "memcache":
		CacheService.Conn = strings.Trim(sec.Key("HOST").String(), "\" ")
	case "":
		return
	default:
		log.Fatal(4, "Unknown cache adapter: %s", CacheService.Adapter)
	}
	CacheService.TTL = sec.Key("ITEM_TTL").MustDuration(16 * time.Hour)

	log.Info("Cache Service Enabled")
}
