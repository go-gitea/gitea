// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package git

import (
	"context"

	"github.com/go-git/go-git/v5/plumbing"
	cgobject "github.com/go-git/go-git/v5/plumbing/object/commitgraph"
)

// CacheCommit will cache the commit from the gitRepository
func (repo *Repository) CacheCommit(ctx context.Context, c *Commit) error {
	if repo.LastCommitCache == nil {
		return nil
	}
	commitNodeIndex, _ := repo.CommitNodeIndex()

	index, err := commitNodeIndex.Get(plumbing.Hash(c.ID.RawValue()))
	if err != nil {
		return err
	}

	return repo.recursiveCache(ctx, c, index, NewTree(repo, c.TreeID), "", 1)
}

func (repo *Repository) recursiveCache(ctx context.Context, c *Commit, index cgobject.CommitNode, tree *Tree, treePath string, level int) error {
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

	commits, err := GetLastCommitForPaths(ctx, repo.LastCommitCache, index, treePath, entryPaths)
	if err != nil {
		return err
	}

	for entry := range commits {
		if entryMap[entry].IsDir() {
			subTree, err := tree.SubTree(entry)
			if err != nil {
				return err
			}
			if err := repo.recursiveCache(ctx, c, index, subTree, entry, level-1); err != nil {
				return err
			}
		}
	}

	return nil
}
