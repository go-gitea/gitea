// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"crypto/sha256"
	"fmt"

	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

func getCacheKey(repoPath, commitID, entryPath string) string {
	hashBytes := sha256.Sum256(fmt.Appendf(nil, "%s:%s:%s", repoPath, commitID, entryPath))
	return fmt.Sprintf("last_commit:%x", hashBytes)
}

// LastCommitCache represents a cache to store last commit
type LastCommitCache struct {
	repoPath    string
	ttl         func() int64
	repo        *Repository
	commitCache map[string]*Commit
	cache       cache.StringCache
}

// NewLastCommitCache creates a new last commit cache for repo
func NewLastCommitCache(count int64, repoPath string, gitRepo *Repository, cache cache.StringCache) *LastCommitCache {
	if cache == nil {
		return nil
	}
	if count < setting.CacheService.LastCommit.CommitsCount {
		return nil
	}

	return &LastCommitCache{
		repoPath: repoPath,
		repo:     gitRepo,
		ttl:      setting.LastCommitCacheTTLSeconds,
		cache:    cache,
	}
}

// Put put the last commit id with commit and entry path
func (c *LastCommitCache) Put(ref, entryPath, commitID string) error {
	if c == nil || c.cache == nil {
		return nil
	}
	log.Debug("LastCommitCache save: [%s:%s:%s]", ref, entryPath, commitID)
	return c.cache.Put(getCacheKey(c.repoPath, ref, entryPath), commitID, c.ttl())
}

// Get gets the last commit information by commit id and entry path
func (c *LastCommitCache) Get(ref, entryPath string) (*Commit, error) {
	if c == nil || c.cache == nil {
		return nil, nil
	}

	commitID, ok := c.cache.Get(getCacheKey(c.repoPath, ref, entryPath))
	if !ok || commitID == "" {
		return nil, nil
	}

	log.Debug("LastCommitCache hit level 1: [%s:%s:%s]", ref, entryPath, commitID)
	if c.commitCache != nil {
		if commit, ok := c.commitCache[commitID]; ok {
			log.Debug("LastCommitCache hit level 2: [%s:%s:%s]", ref, entryPath, commitID)
			return commit, nil
		}
	}

	commit, err := c.repo.GetCommit(commitID)
	if err != nil {
		return nil, err
	}
	if c.commitCache == nil {
		c.commitCache = make(map[string]*Commit)
	}
	c.commitCache[commitID] = commit
	return commit, nil
}

// GetCommitByPath gets the last commit for the entry in the provided commit
func (c *LastCommitCache) GetCommitByPath(commitID, entryPath string) (*Commit, error) {
	sha, err := NewIDFromString(commitID)
	if err != nil {
		return nil, err
	}

	lastCommit, err := c.Get(sha.String(), entryPath)
	if err != nil || lastCommit != nil {
		return lastCommit, err
	}

	lastCommit, err = c.repo.getCommitByPathWithID(sha, entryPath)
	if err != nil {
		return nil, err
	}

	if err := c.Put(commitID, entryPath, lastCommit.ID.String()); err != nil {
		log.Error("Unable to cache %s as the last commit for %q in %s %s. Error %v", lastCommit.ID.String(), entryPath, commitID, c.repoPath, err)
	}

	return lastCommit, nil
}
