// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"context"
	"fmt"
	"io"
	"path"
	"sort"

	"code.gitea.io/gitea/modules/log"
)

// GetCommitsInfo gets information of all commits that are corresponding to these entries
func (tes Entries) GetCommitsInfo(ctx context.Context, commit *Commit, treePath string) ([]CommitInfo, *Commit, error) {
	entryPaths := make([]string, len(tes)+1)
	// Get the commit for the treePath itself
	entryPaths[0] = ""
	for i, entry := range tes {
		entryPaths[i+1] = entry.Name()
	}

	var err error

	var revs map[string]*Commit
	if commit.repo.LastCommitCache != nil {
		var unHitPaths []string
		revs, unHitPaths, err = getLastCommitForPathsByCache(commit.ID.String(), treePath, entryPaths, commit.repo.LastCommitCache)
		if err != nil {
			return nil, nil, err
		}
		if len(unHitPaths) > 0 {
			sort.Strings(unHitPaths)
			commits, err := GetLastCommitForPaths(ctx, commit, treePath, unHitPaths)
			if err != nil {
				return nil, nil, err
			}

			for pth, found := range commits {
				revs[pth] = found
			}
		}
	} else {
		sort.Strings(entryPaths)
		revs, err = GetLastCommitForPaths(ctx, commit, treePath, entryPaths)
	}
	if err != nil {
		return nil, nil, err
	}

	commitsInfo := make([]CommitInfo, len(tes))
	for i, entry := range tes {
		commitsInfo[i] = CommitInfo{
			Entry: entry,
		}

		// Check if we have found a commit for this entry in time
		if entryCommit, ok := revs[entry.Name()]; ok {
			commitsInfo[i].Commit = entryCommit
		} else {
			log.Debug("missing commit for %s", entry.Name())
		}

		// If the entry if a submodule add a submodule file for this
		if entry.IsSubModule() {
			subModuleURL := ""
			var fullPath string
			if len(treePath) > 0 {
				fullPath = treePath + "/" + entry.Name()
			} else {
				fullPath = entry.Name()
			}
			if subModule, err := commit.GetSubModule(fullPath); err != nil {
				return nil, nil, err
			} else if subModule != nil {
				subModuleURL = subModule.URL
			}
			subModuleFile := NewCommitSubmoduleFile(subModuleURL, entry.ID.String())
			commitsInfo[i].SubmoduleFile = subModuleFile
		}
	}

	// Retrieve the commit for the treePath itself (see above). We basically
	// get it for free during the tree traversal and it's used for listing
	// pages to display information about newest commit for a given path.
	var treeCommit *Commit
	var ok bool
	if treePath == "" {
		treeCommit = commit
	} else if treeCommit, ok = revs[""]; ok {
		treeCommit.repo = commit.repo
	}
	return commitsInfo, treeCommit, nil
}

func getLastCommitForPathsByCache(commitID, treePath string, paths []string, cache *LastCommitCache) (map[string]*Commit, []string, error) {
	var unHitEntryPaths []string
	results := make(map[string]*Commit)
	for _, p := range paths {
		lastCommit, err := cache.Get(commitID, path.Join(treePath, p))
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

// GetLastCommitForPaths returns last commit information
func GetLastCommitForPaths(ctx context.Context, commit *Commit, treePath string, paths []string) (map[string]*Commit, error) {
	// We read backwards from the commit to obtain all of the commits
	revs, err := WalkGitLog(ctx, commit.repo, commit, treePath, paths...)
	if err != nil {
		return nil, err
	}

	batchStdinWriter, batchReader, cancel, err := commit.repo.CatFileBatch(ctx)
	if err != nil {
		return nil, err
	}
	defer cancel()

	commitsMap := map[string]*Commit{}
	commitsMap[commit.ID.String()] = commit

	commitCommits := map[string]*Commit{}
	for path, commitID := range revs {
		c, ok := commitsMap[commitID]
		if ok {
			commitCommits[path] = c
			continue
		}

		if len(commitID) == 0 {
			continue
		}

		_, err := batchStdinWriter.Write([]byte(commitID + "\n"))
		if err != nil {
			return nil, err
		}
		_, typ, size, err := ReadBatchLine(batchReader)
		if err != nil {
			return nil, err
		}
		if typ != "commit" {
			if err := DiscardFull(batchReader, size+1); err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("unexpected type: %s for commit id: %s", typ, commitID)
		}
		c, err = CommitFromReader(commit.repo, MustIDFromString(commitID), io.LimitReader(batchReader, size))
		if err != nil {
			return nil, err
		}
		if _, err := batchReader.Discard(1); err != nil {
			return nil, err
		}
		commitCommits[path] = c
	}

	return commitCommits, nil
}
