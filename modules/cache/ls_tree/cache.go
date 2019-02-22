package ls_tree

import (
	"fmt"

	"code.gitea.io/git"

	"github.com/go-macaron/cache"
)

type lsTreeCache struct {
	mc      cache.Cache
	timeout int64 // seconds
}

func getKey(repoPath, ref string) string {
	return fmt.Sprintf("%s:%s", repoPath, ref)
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
