// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cache

import (
	"fmt"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"

	mc "gitea.com/macaron/cache"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
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

// Get get the last commit information by commit id and entry path
func (c LastCommitCache) Get(ref, entryPath string) (*object.Commit, error) {
	v := c.Cache.Get(fmt.Sprintf("last_commit:%s:%s:%s", c.repoPath, ref, entryPath))
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
	return c.Cache.Put(fmt.Sprintf("last_commit:%s:%s:%s", c.repoPath, ref, entryPath), commitID, c.ttl)
}
