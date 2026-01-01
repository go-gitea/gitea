// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package gitrepo

import (
	"context"
	"sort"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"github.com/go-git/go-git/v5/plumbing"
)

// GetBranches returns branches from the repository, skipping "skip" initial branches and
// returning at most "limit" branches, or all branches if "limit" is 0.
// Branches are returned with sort of `-committerdate` as the nogogit
// implementation. This requires full fetch, sort and then the
// skip/limit applies later as gogit returns in undefined order.
func GetBranchNames(ctx context.Context, repo Repository, skip, limit int) ([]string, int, error) {
	type BranchData struct {
		name          string
		committerDate int64
	}
	var branchData []BranchData

	gitRepo, closer, err := RepositoryFromContextOrOpen(ctx, repo)
	if err != nil {
		return nil, 0, err
	}
	defer closer.Close()

	branchIter, err := gitRepo.GoGitRepo().Branches()
	if err != nil {
		return nil, 0, err
	}
	defer branchIter.Close()

	_ = branchIter.ForEach(func(branch *plumbing.Reference) error {
		obj, err := gitRepo.GoGitRepo().CommitObject(branch.Hash())
		if err != nil {
			// skip branch if can't find commit
			return nil
		}

		branchData = append(branchData, BranchData{strings.TrimPrefix(branch.Name().String(), git.BranchPrefix), obj.Committer.When.Unix()})
		return nil
	})

	sort.Slice(branchData, func(i, j int) bool {
		return !(branchData[i].committerDate < branchData[j].committerDate)
	})

	var branchNames []string
	maxPos := len(branchData)
	if limit > 0 {
		maxPos = min(skip+limit, maxPos)
	}
	for i := skip; i < maxPos; i++ {
		branchNames = append(branchNames, branchData[i].name)
	}

	return branchNames, len(branchData), nil
}
