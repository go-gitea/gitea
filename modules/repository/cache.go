// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"path"
	"strings"

	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"

	cgobject "github.com/go-git/go-git/v5/plumbing/object/commitgraph"
)

func recusiveCache(gitRepo *git.Repository, c cgobject.CommitNode, tree *git.Tree, treePath string, ca *cache.LastCommitCache, level int) error {
	if level == 0 {
		return nil
	}

	entries, err := tree.ListEntries()
	if err != nil {
		return err
	}

	entryPaths := make([]string, len(entries), len(entries))
	for i, entry := range entries {
		entryPaths[i] = entry.Name()
	}

	commits, err := git.GetLastCommitForPaths(c, treePath, entryPaths)
	if err != nil {
		return err
	}

	for entry, cm := range commits {
		ca.Put(c.ID().String(), path.Join(treePath, entry), cm.ID().String())

		subTree, err := tree.SubTree(entry)
		if err != nil {
			return err
		}
		if err := recusiveCache(gitRepo, c, subTree, entry, ca, level-1); err != nil {
			return err
		}
	}

	return nil
}

func getRefName(fullRefName string) string {
	if strings.HasPrefix(fullRefName, git.TagPrefix) {
		return fullRefName[len(git.TagPrefix):]
	} else if strings.HasPrefix(fullRefName, git.BranchPrefix) {
		return fullRefName[len(git.BranchPrefix):]
	}
	return ""
}

// CacheRef cachhe last commit information of the branch or the tag
func CacheRef(gitRepo *git.Repository, fullRefName string) error {
	if !setting.CacheService.LastCommit.Enabled {
		return nil
	}

	commit, err := gitRepo.GetCommit(fullRefName)
	if err != nil {
		return err
	}

	commitsCount, err := cache.GetInt64(r.Repository.GetCommitsCountCacheKey(getRefName(fullRefName), true), func() (int64, error) {
		return commit.CommitsCount()
	})
	if err != nil {
		return err
	}
	if commitsCount < setting.CacheService.LastCommit.CommitsCount {
		return nil
	}

	commitNodeIndex, _ := gitRepo.CommitNodeIndex()

	c, err := commitNodeIndex.Get(commit.ID)
	if err != nil {
		return err
	}

	ca := cache.NewLastCommitCache("", gitRepo, int64(setting.CacheService.LastCommit.TTL.Seconds()))

	return recusiveCache(gitRepo, c, &commit.Tree, "", ca, 3)
}
