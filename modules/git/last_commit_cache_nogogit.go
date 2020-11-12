// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build nogogit

package git

import (
	"crypto/sha256"
	"fmt"
	"path"
)

// Cache represents a caching interface
type Cache interface {
	// Put puts value into cache with key and expire time.
	Put(key string, val interface{}, timeout int64) error
	// Get gets cached value by given key.
	Get(key string) interface{}
}

// LastCommitCache represents a cache to store last commit
type LastCommitCache struct {
	repoPath    string
	ttl         int64
	repo        *Repository
	commitCache map[string]*Commit
	cache       Cache
}

// NewLastCommitCache creates a new last commit cache for repo
func NewLastCommitCache(repoPath string, gitRepo *Repository, ttl int64, cache Cache) *LastCommitCache {
	if cache == nil {
		return nil
	}
	return &LastCommitCache{
		repoPath:    repoPath,
		repo:        gitRepo,
		commitCache: make(map[string]*Commit),
		ttl:         ttl,
		cache:       cache,
	}
}

func (c *LastCommitCache) getCacheKey(repoPath, ref, entryPath string) string {
	hashBytes := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s", repoPath, ref, entryPath)))
	return fmt.Sprintf("last_commit:%x", hashBytes)
}

// Get get the last commit information by commit id and entry path
func (c *LastCommitCache) Get(ref, entryPath string) (interface{}, error) {
	v := c.cache.Get(c.getCacheKey(c.repoPath, ref, entryPath))
	if vs, ok := v.(string); ok {
		log("LastCommitCache hit level 1: [%s:%s:%s]", ref, entryPath, vs)
		if commit, ok := c.commitCache[vs]; ok {
			log("LastCommitCache hit level 2: [%s:%s:%s]", ref, entryPath, vs)
			return commit, nil
		}
		id, err := c.repo.ConvertToSHA1(vs)
		if err != nil {
			return nil, err
		}
		commit, err := c.repo.getCommit(id)
		if err != nil {
			return nil, err
		}
		c.commitCache[vs] = commit
		return commit, nil
	}
	return nil, nil
}

// Put put the last commit id with commit and entry path
func (c *LastCommitCache) Put(ref, entryPath, commitID string) error {
	log("LastCommitCache save: [%s:%s:%s]", ref, entryPath, commitID)
	return c.cache.Put(c.getCacheKey(c.repoPath, ref, entryPath), commitID, c.ttl)
}

// CacheCommit will cache the commit from the gitRepository
func (c *LastCommitCache) CacheCommit(gitRepo *Repository, commit *Commit) error {
	return c.recursiveCache(gitRepo, commit, &commit.Tree, "", 1)
}

func (c *LastCommitCache) recursiveCache(gitRepo *Repository, commit *Commit, tree *Tree, treePath string, level int) error {
	if level == 0 {
		return nil
	}

	entries, err := tree.ListEntries()
	if err != nil {
		return err
	}

	entryPaths := make([]string, len(entries))
	entryMap := make(map[string]*TreeEntry)
	for i, entry := range entries {
		entryPaths[i] = entry.Name()
		entryMap[entry.Name()] = entry
	}

	commits, err := GetLastCommitForPaths(commit, treePath, entryPaths)
	if err != nil {
		return err
	}

	for i, entryCommit := range commits {
		entry := entryPaths[i]
		if err := c.Put(commit.ID.String(), path.Join(treePath, entryPaths[i]), entryCommit.ID.String()); err != nil {
			return err
		}
		if entryMap[entry].IsDir() {
			subTree, err := tree.SubTree(entry)
			if err != nil {
				return err
			}
			if err := c.recursiveCache(gitRepo, commit, subTree, entry, level-1); err != nil {
				return err
			}
		}
	}

	return nil
}
