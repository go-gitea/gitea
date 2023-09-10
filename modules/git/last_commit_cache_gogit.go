// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package git

import (
	"context"

	cgobject "github.com/go-git/go-git/v5/plumbing/object/commitgraph"
)

// CacheCommit will cache the commit from the gitRepository
func (c *Commit) CacheCommit(ctx context.Context) error {
	if c.repo.LastCommitCache == nil {
		return nil
	}
	commitNodeIndex, _ := c.repo.CommitNodeIndex()

	index, err := commitNodeIndex.Get(c.ID)
	if err != nil {
		return err
	}

	return c.recursiveCache(ctx, index, &c.Tree, "", 1)
}

func (c *Commit) recursiveCache(ctx context.Context, index cgobject.CommitNode, tree *Tree, treePath string, level int) error {
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

	commits, err := GetLastCommitForPaths(ctx, c.repo.LastCommitCache, index, treePath, entryPaths)
	if err != nil {
		return err
	}

	for entry := range commits {
		if entryMap[entry].IsDir() {
			subTree, err := tree.SubTree(entry)
			if err != nil {
				return err
			}
			if err := c.recursiveCache(ctx, index, subTree, entry, level-1); err != nil {
				return err
			}
		}
	}

	return nil
}
