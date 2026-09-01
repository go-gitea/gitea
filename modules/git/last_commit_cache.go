// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"

	"gitea.dev/modules/cache"
	"gitea.dev/modules/log"
)

func getCacheKey(repo RepositoryFacade, commitID, entryPath string) string {
	return cache.SafeCacheKey(fmt.Sprintf("git-last-commit:%s:%s", repo.GitRepoManagedID(), commitID), entryPath)
}

// LastCommitCache represents a cache to store last commit
type LastCommitCache struct {
	ttlFn       func() int64
	repo        *Repository
	commitCache map[string]*Commit
	cache       cache.StringCache
}

// Put puts the last commit id with commit and entry path
func (c *LastCommitCache) Put(ref, entryPath, commitID string) error {
	log.Debug("LastCommitCache save: [%s:%s:%s]", ref, entryPath, commitID)
	return c.cache.Put(getCacheKey(c.repo, ref, entryPath), commitID, c.ttlFn())
}

// Get gets the last commit information by commit id and entry path
func (c *LastCommitCache) Get(ctx context.Context, ref, entryPath string) (*Commit, error) {
	lastCommitID, ok := c.cache.Get(getCacheKey(c.repo, ref, entryPath))
	if !ok || lastCommitID == "" {
		return nil, nil //nolint:nilnil // return nil when cache miss
	}

	log.Debug("LastCommitCache hit level 1: [%s:%s:%s]", ref, entryPath, lastCommitID)
	if lastCommit, ok := c.commitCache[lastCommitID]; ok {
		log.Debug("LastCommitCache hit level 2: [%s:%s:%s]", ref, entryPath, lastCommitID)
		return lastCommit, nil
	}

	lastCommit, err := c.repo.GetCommit(ctx, lastCommitID)
	if err != nil {
		return nil, err
	}
	if c.commitCache == nil {
		c.commitCache = make(map[string]*Commit)
	}
	c.commitCache[lastCommitID] = lastCommit
	return lastCommit, nil
}

// GetCommitByPath gets the last commit for the entry in the provided commit
func (c *LastCommitCache) GetCommitByPath(ctx context.Context, entryCommitID ObjectID, entryPath string) (*Commit, error) {
	entryCommitIDStr := entryCommitID.String()
	lastCommit, err := c.Get(ctx, entryCommitIDStr, entryPath)
	if err != nil || lastCommit != nil {
		return lastCommit, err
	}

	lastCommit, err = c.repo.getCommitByPathWithID(ctx, entryCommitID, entryPath)
	if err != nil {
		return nil, err
	}

	if err := c.Put(entryCommitIDStr, entryPath, lastCommit.ID.String()); err != nil {
		log.Error("Unable to cache %s as the last commit for %q in %s %s. Error %v", lastCommit.ID.String(), entryPath, entryCommitID, c.repo.LogString(), err)
	}

	return lastCommit, nil
}
