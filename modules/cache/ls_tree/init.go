package ls_tree

import (
	"errors"
	"time"

	"code.gitea.io/git"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/setting"
)

var (
	// LastCommitCache defines globally last commit cache object
	Cache git.LsTreeCache
)

// NewContext init
func NewContext() error {
	if cache.Cache != nil {
		Cache = &lsTreeCache{
			mc:      cache.Cache,
			timeout: int64(setting.CacheService.TTL / time.Second),
		}
		return nil
	}

	return errors.New("unsupported cache type")
}
