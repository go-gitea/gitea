// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gogit

import (
	"crypto/sha256"
	"fmt"
	"path"

	"code.gitea.io/gitea/modules/git/service"
	"code.gitea.io/gitea/modules/log"

	"github.com/go-git/go-git/v5/plumbing/object"
	cgobject "github.com/go-git/go-git/v5/plumbing/object/commitgraph"
)

// LastCommitCache represents a cache to store last commit
type LastCommitCache struct {
	ttl         int64
	repo        service.Repository
	commitCache map[string]*object.Commit
	cache       service.Cache
}

// Get get the last commit information by commit id and entry path
func (c *LastCommitCache) Get(ref, entryPath string) (interface{}, error) {
	v := c.cache.Get(c.getCacheKey(c.repo.Path(), ref, entryPath))
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
		gitRepo, err := GetGoGitRepo(c.repo)
		if err != nil {
			return nil, err
		}
		commit, err := gitRepo.CommitObject(ToPlumbingHash(id))
		if err != nil {
			return nil, err
		}
		c.commitCache[vs] = commit
		return commit, nil
	}
	return nil, nil
}

// CacheCommit will cache the commit from the gitRepository
func (c *LastCommitCache) CacheCommit(commit service.Commit) error {
	repo, ok := commit.Repository().(*Repository)
	if !ok {
		return fmt.Errorf("not a gogit repository")
	}

	commitNodeIndex, _ := repo.CommitNodeIndex()

	index, err := commitNodeIndex.Get(ToPlumbingHash(commit.ID()))
	if err != nil {
		return err
	}

	return c.recursiveCache(index, commit.Tree(), "", 1)
}

func (c *LastCommitCache) recursiveCache(index cgobject.CommitNode, tree service.Tree, treePath string, level int) error {
	if level == 0 {
		return nil
	}

	entries, err := tree.ListEntries()
	if err != nil {
		return err
	}

	entryPaths := make([]string, len(entries))
	entryMap := make(map[string]service.TreeEntry)
	for i, entry := range entries {
		entryPaths[i] = entry.Name()
		entryMap[entry.Name()] = entry
	}

	commits, err := GetLastCommitForPaths(index, treePath, entryPaths)
	if err != nil {
		return err
	}

	for entry, cm := range commits {
		if err := c.Put(index.ID().String(), path.Join(treePath, entry), cm.ID().String()); err != nil {
			return err
		}
		if entryMap[entry].Mode().IsDir() {
			subTree, err := tree.SubTree(entry)
			if err != nil {
				return err
			}
			if err := c.recursiveCache(index, subTree, entry, level-1); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *LastCommitCache) getCacheKey(repoPath, ref, entryPath string) string {
	hashBytes := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s", repoPath, ref, entryPath)))
	return fmt.Sprintf("last_commit:%x", hashBytes)
}

// Put put the last commit id with commit and entry path
func (c *LastCommitCache) Put(ref, entryPath, commitID string) error {
	log.Trace("LastCommitCache save: [%s:%s:%s]", ref, entryPath, commitID)
	return c.cache.Put(c.getCacheKey(c.repo.Path(), ref, entryPath), commitID, c.ttl)
}
