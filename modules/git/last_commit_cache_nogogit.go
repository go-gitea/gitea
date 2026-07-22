// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"context"
)

// CacheCommit will cache the commit from the gitRepository
func (c *Commit) CacheCommit(ctx context.Context, gitRepo *Repository) error {
	if gitRepo.LastCommitCache == nil {
		return nil
	}
	return c.recursiveCache(ctx, gitRepo, c.Tree(), "", 1)
}

func (c *Commit) recursiveCache(ctx context.Context, gitRepo *Repository, tree *Tree, treePath string, level int) error {
	if level == 0 {
		return nil
	}
	entries, err := tree.ListEntries(ctx, gitRepo)
	if err != nil {
		return err
	}

	entryPaths := make([]string, len(entries))
	for i, entry := range entries {
		entryPaths[i] = entry.Name()
	}

	_, err = walkGitLog(ctx, gitRepo, c, treePath, entryPaths...)
	if err != nil {
		return err
	}

	for _, treeEntry := range entries {
		// entryMap won't contain "" therefore skip this.
		if treeEntry.IsDir() {
			subTree, err := tree.SubTree(ctx, gitRepo, treeEntry.Name())
			if err != nil {
				return err
			}
			if err := c.recursiveCache(ctx, gitRepo, subTree, treeEntry.Name(), level-1); err != nil {
				return err
			}
		}
	}

	return nil
}
