// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"context"
)

// GetLastCommitForPaths returns last commit information
func GetLastCommitForPaths(ctx context.Context, gitRepo *Repository, commit *Commit, treePath string, paths []string) (map[string]*Commit, error) {
	// We read backwards from the commit to obtain all of the commits
	revs, err := walkGitLog(ctx, gitRepo, commit, treePath, paths...)
	if err != nil {
		return nil, err
	}

	commitsMap := map[string]*Commit{}
	commitsMap[commit.ID.String()] = commit

	commitCommits := map[string]*Commit{}
	for path, commitID := range revs {
		if len(commitID) == 0 {
			continue
		}

		c, ok := commitsMap[commitID]
		if ok {
			commitCommits[path] = c
			continue
		}

		c, err := gitRepo.GetCommit(ctx, commitID) // Ensure the commit exists in the repository
		if err != nil {
			return nil, err
		}
		commitCommits[path] = c
	}

	return commitCommits, nil
}
