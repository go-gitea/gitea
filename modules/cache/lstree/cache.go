// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package lstree

import (
	"fmt"

	"code.gitea.io/git"

	"github.com/go-macaron/cache"
)

type lsTreeCache struct {
	mc      cache.Cache
	timeout int64 // seconds
}

func getKey(repoPath, treeIsh string) string {
	return fmt.Sprintf("%s:%s", treeIsh, repoPath)
}

func (c *lsTreeCache) Get(repoPath, id string) (git.Entries, error) {
	res := c.mc.Get(getKey(repoPath, id))
	if res == nil {
		return nil, nil
	}
	return res.(git.Entries), nil
}

func (c *lsTreeCache) Put(repoPath, id string, entries git.Entries) error {
	return c.mc.Put(getKey(repoPath, id), entries, c.timeout)
}
