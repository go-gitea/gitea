// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"crypto/sha256"
	"fmt"
	"path"

	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

func getCacheKey(repoPath, commitID, entryPath string) string {
	hashBytes := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s", repoPath, commitID, entryPath)))
	return fmt.Sprintf("last_commit:%x", hashBytes)
}

// lastCommitCache represents a cache to store last commit
type lastCommitCache struct {
	repoPath string
	repo     *Repository
	ttl      func() int64
	cache    cache.StringCache
}

// newLastCommitCache creates a new last commit cache for repo
func newLastCommitCache(repoPath string, gitRepo *Repository, cache cache.StringCache) *lastCommitCache {
	if cache == nil {
		return nil
	}

	return &lastCommitCache{
		repoPath: repoPath,
		repo:     gitRepo,
		ttl:      setting.LastCommitCacheTTLSeconds,
		cache:    cache,
	}
}

// Put put the last commit id with commit and entry path
func (c *lastCommitCache) Put(ref, entryPath, commitID string) error {
	if c == nil || c.cache == nil {
		return nil
	}
	log.Debug("LastCommitCache save: [%s:%s:%s]", ref, entryPath, commitID)
	return c.cache.Put(getCacheKey(c.repoPath, ref, entryPath), commitID, c.ttl())
}

// Get gets the last commit information by commit id and entry path
func (c *lastCommitCache) Get(ref, entryPath string) (*Commit, error) {
	if c == nil || c.cache == nil {
		return nil, nil
	}

	commitID, ok := c.cache.Get(getCacheKey(c.repoPath, ref, entryPath))
	if !ok || commitID == "" {
		return nil, nil
	}

	log.Debug("LastCommitCache hit: [%s:%s:%s]", ref, entryPath, commitID)
	return c.repo.GetCommit(commitID)
}

// GetCommitByPath gets the last commit for the entry in the provided commit
func (c *lastCommitCache) GetCommitByPath(commitID, entryPath string) (*Commit, error) {
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

func (c *lastCommitCache) getLastCommitForPathsByCache(commitID, treePath string, paths []string) (map[string]*Commit, []string, error) {
	var unHitEntryPaths []string
	results := make(map[string]*Commit)
	for _, p := range paths {
		lastCommit, err := c.Get(commitID, path.Join(treePath, p))
		if err != nil {
			return nil, nil, err
		}
		if lastCommit != nil {
			results[p] = lastCommit
			continue
		}

		unHitEntryPaths = append(unHitEntryPaths, p)
	}

	return results, unHitEntryPaths, nil
}

func (repo *Repository) CacheCommit(ctx context.Context, commit *Commit) error {
	return repo.lastCommitCache.CacheCommit(ctx, commit)
}
