// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package lastcommit

import (
	"code.gitea.io/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

var (
	// LastCommitCache defines gloablly last commit cache object
	LastCommitCache git.LastCommitCache
)

// NewContext init
func NewContext() error {
	var err error
	switch setting.Git.LastCommitCache.Type {
	case "memory":
		LastCommitCache = &MemoryCache{}
	case "boltdb":
		LastCommitCache, err = NewBoltDBCache(setting.Git.LastCommitCache.DataPath)
	}
	if err == nil {
		log.Info("Last Commit Cache %s Enabled", setting.Git.LastCommitCache.Type)
	}
	return err
}
