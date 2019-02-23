// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package lstree

import (
	"fmt"
	"time"

	"code.gitea.io/git"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

var (
	// Cache defines globally tree entries cache
	Cache git.LsTreeCache
)

// NewContext init
func NewContext() error {
	var err error
	switch setting.CacheService.LsTree.Type {
	case "default":
		if cache.Cache != nil {
			Cache = &lsTreeCache{
				mc:      cache.Cache,
				timeout: int64(setting.CacheService.TTL / time.Second),
			}
		} else {
			log.Warn("Ls Tree Cache Enabled but Cache Service not Configed Well")
			return nil
		}
	case "memory":
		Cache = &MemoryCache{}
	case "boltdb":
		Cache, err = NewBoltDBCache(setting.CacheService.LsTree.ConnStr)
	case "redis":
		addrs, pass, dbIdx, err := parseConnStr(setting.CacheService.LsTree.ConnStr)
		if err != nil {
			return err
		}

		Cache, err = NewRedisCache(addrs, pass, dbIdx)
	case "":
		return nil
	default:
		return fmt.Errorf("Unsupported ls tree cache type: %s", setting.CacheService.LsTree.Type)
	}
	if err == nil {
		log.Info("Ls Tree Cache %s Enabled", setting.CacheService.LsTree.Type)
	}
	return err
}
