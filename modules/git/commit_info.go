// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"maps"
	"path"
	"time"

	"gitea.dev/modules/log"
	"gitea.dev/modules/util"
)

// CommitInfo describes the first commit with the provided entry
type CommitInfo struct {
	Entry         *TreeEntry
	Commit        *Commit
	SubmoduleFile *CommitSubmoduleFile
}

func GetCommitInfoSubmoduleFile(ctx context.Context, repoLink, fullPath string, gitRepo *Repository, commit *Commit, refCommitID ObjectID) (*CommitSubmoduleFile, error) {
	submodule, err := commit.GetSubModule(ctx, gitRepo, fullPath)
	if err != nil {
		return nil, err
	}
	if submodule == nil {
		// unable to find submodule from ".gitmodules" file
		return NewCommitSubmoduleFile(repoLink, fullPath, "", refCommitID.String()), nil
	}
	return NewCommitSubmoduleFile(repoLink, fullPath, submodule.URL, refCommitID.String()), nil
}

func getLastCommitForPathsWithTimeout(ctx context.Context, timeout time.Duration, gitRepo *Repository, commit *Commit, treePath string, entryNames []string) (map[string]*Commit, error) {
	// Actually using "timeout ctx" here is not ideal: it creates a new git cat-file batch process
	// (the other one uses the request's ctx and is cached by the gitRepo).
	// Ideally, the underlying functions only need to return in predictable time,
	// so it should use the shared (cached) git cat-file batcher from the request context and just use "time.After" for deadline.
	// However, it's difficult to make it right at the moment.
	ctxInner := ctx
	if timeout > 0 {
		var cancel context.CancelFunc
		ctxInner, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	revs, err := GetLastCommitForPaths(ctxInner, gitRepo, commit, treePath, entryNames)
	if err != nil && ctxInner.Err() == nil {
		// only return the err if context is not canceled
		return nil, err
	}
	return revs, nil
}

// GetCommitsInfo gets information of all commits that are corresponding to these entries
func (tes Entries) GetCommitsInfo(ctx context.Context, timeout time.Duration, repoLink string, gitRepo *Repository, commit *Commit, treePath string) (commitsInfo []CommitInfo, treeLastCommit *Commit, err error) {
	entryNames := make([]string, 0, len(tes)+1)
	entryNames = append(entryNames, "") // include the commit for the treePath itself
	for _, entry := range tes {
		entryNames = append(entryNames, entry.Name())
	}

	revs, remainingEntryNames, err := getLastCommitForPathsByCache(ctx, commit.ID.String(), treePath, entryNames, gitRepo.LastCommitCache)
	if err != nil {
		return nil, nil, err
	}

	if len(remainingEntryNames) > 0 {
		remainingRevs, err := getLastCommitForPathsWithTimeout(ctx, timeout, gitRepo, commit, treePath, remainingEntryNames)
		if err != nil {
			return nil, nil, err
		}
		maps.Copy(revs, remainingRevs)
	}

	for _, entry := range tes {
		ci := CommitInfo{Entry: entry, Commit: revs[entry.Name()]}
		if ci.Commit == nil {
			log.Debug("missing commit for %s", entry.Name())
		}
		// If the entry is a submodule, add a submodule file for this
		if entry.IsSubModule() {
			ci.SubmoduleFile, err = GetCommitInfoSubmoduleFile(ctx, repoLink, path.Join(treePath, entry.Name()), gitRepo, commit, entry.ID)
			if err != nil {
				return nil, nil, err
			}
		}
		commitsInfo = append(commitsInfo, ci)
	}

	// Retrieve the commit for the treePath itself (see above). We basically
	// get it for free during the tree traversal, and it's used for listing
	// pages to display information about the newest commit for a given path.
	treeLastCommit = util.Iif(treePath == "", commit, revs[""])
	return commitsInfo, treeLastCommit, nil
}

func getLastCommitForPathsByCache(ctx context.Context, commitID, treePath string, paths []string, cache *LastCommitCache) (map[string]*Commit, []string, error) {
	var unHitEntryPaths []string
	results := make(map[string]*Commit)
	for _, p := range paths {
		lastCommit, err := cache.Get(ctx, commitID, path.Join(treePath, p))
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
