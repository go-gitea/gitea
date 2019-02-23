// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package lstree

import (
	"sync"

	"code.gitea.io/git"
)

var (
	_ git.LsTreeCache = &MemoryCache{}
)

// MemoryCache implements git.LsTreeCache interface to save the ls tree on memory
type MemoryCache struct {
	entries sync.Map
}

// Get implements git.LastCommitCache
func (c *MemoryCache) Get(repoPath, treeIsh string) (git.Entries, error) {
	v, ok := c.entries.Load(getKey(repoPath, treeIsh))
	if ok {
		return v.(git.Entries), nil
	}
	return nil, nil
}

// Put implements git.LastCommitCache
func (c *MemoryCache) Put(repoPath, treeIsh string, entries git.Entries) error {
	c.entries.Store(getKey(repoPath, treeIsh), entries)
	return nil
}
