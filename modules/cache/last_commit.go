// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cache

import (
	"crypto/sha256"
	"fmt"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"

	mc "gitea.com/macaron/cache"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// LastCommitCache represents a cache to store last commit
type LastCommitCache struct {
	repoPath    string
	ttl         int64
	repo        *git.Repository
	commitCache map[string]*object.Commit
	mc.Cache
}

// NewLastCommitCache creates a new last commit cache for repo
func NewLastCommitCache(repoPath string, gitRepo *git.Repository, ttl int64) *LastCommitCache {
	return &LastCommitCache{
		repoPath:    repoPath,
		repo:        gitRepo,
		commitCache: make(map[string]*object.Commit),
		ttl:         ttl,
		Cache:       conn,
	}
}

func (c LastCommitCache) getCacheKey(repoPath, ref, entryPath string) string {
	hashBytes := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s", repoPath, ref, entryPath)))
	return fmt.Sprintf("last_commit:%x", hashBytes)
}

// Get get the last commit information by commit id and entry path
func (c LastCommitCache) Get(ref, entryPath string) (*object.Commit, error) {
	v := c.Cache.Get(c.getCacheKey(c.repoPath, ref, entryPath))
	if vs, ok := v.(string); ok {
		log.Trace("LastCommitCache hit level 1: [%s:%s:%s]", ref, entryPath, vs)
		if commit, ok := c.commitCache[vs]; ok {
			log.Trace("LastCommitCache hit level 2: [%s:%s:%s]", ref, entryPath, vs)
			return commit, nil
		}
		id, err := c.repo.ConvertToSHA1(vs)
		if err != nil {
			return nil, err
		}
		commit, err := c.repo.GoGitRepo().CommitObject(id)
		if err != nil {
			return nil, err
		}
		c.commitCache[vs] = commit
		return commit, nil
	}
	return nil, nil
}

// Put put the last commit id with commit and entry path
func (c LastCommitCache) Put(ref, entryPath, commitID string) error {
	log.Trace("LastCommitCache save: [%s:%s:%s]", ref, entryPath, commitID)
	return c.Cache.Put(c.getCacheKey(c.repoPath, ref, entryPath), commitID, c.ttl)
}
