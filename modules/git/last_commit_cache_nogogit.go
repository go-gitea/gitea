// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"context"
)

// CacheCommit will cache the commit from the gitRepository
func (c *lastCommitCache) CacheCommit(ctx context.Context, commit *Commit) error {
	return c.recursiveCache(ctx, commit, &commit.Tree, "", 1)
}

func (c *lastCommitCache) recursiveCache(ctx context.Context, commit *Commit, tree *Tree, treePath string, level int) error {
	if level == 0 {
		return nil
	}

	entries, err := tree.ListEntries()
	if err != nil {
		return err
	}

	entryPaths := make([]string, len(entries))
	for i, entry := range entries {
		entryPaths[i] = entry.Name()
	}

	_, err = WalkGitLog(ctx, c.repo, commit, treePath, entryPaths...)
	if err != nil {
		return err
	}

	for _, treeEntry := range entries {
		// entryMap won't contain "" therefore skip this.
		if treeEntry.IsDir() {
			subTree, err := tree.SubTree(treeEntry.Name())
			if err != nil {
				return err
			}
			if err := c.recursiveCache(ctx, commit, subTree, treeEntry.Name(), level-1); err != nil {
				return err
			}
		}
	}

	return nil
}
