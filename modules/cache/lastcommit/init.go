// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package lastcommit

import (
	"time"

	"code.gitea.io/git"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

var (
	// LastCommitCache defines globally last commit cache object
	LastCommitCache git.LastCommitCache
)

// NewContext init
func NewContext() error {
	if cache.Cache != nil && setting.Git.LastCommitCache.UseDefaultCache {
		LastCommitCache = &lastCommitCache{
			mc:      cache.Cache,
			timeout: int64(setting.CacheService.TTL / time.Second),
		}
		return nil
	}

	var err error
	switch setting.Git.LastCommitCache.Type {
	case "memory":
		LastCommitCache = &MemoryCache{}
	case "boltdb":
		LastCommitCache, err = NewBoltDBCache(setting.Git.LastCommitCache.ConnStr)
	case "redis":
		addrs, pass, dbIdx, err := parseConnStr(setting.Git.LastCommitCache.ConnStr)
		if err != nil {
			return err
		}

		LastCommitCache, err = NewRedisCache(addrs, pass, dbIdx)
	}
	if err == nil {
		log.Info("Last Commit Cache %s Enabled", setting.Git.LastCommitCache.Type)
	}
	return err
}
