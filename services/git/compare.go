// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/util"
)

// CompareInfo represents needed information for comparing references.
type CompareInfo struct {
	BaseRepo         *repo_model.Repository
	BaseRef          git.RefName
	BaseCommitID     string
	HeadRepo         *repo_model.Repository
	HeadGitRepo      *git.Repository
	HeadRef          git.RefName
	HeadCommitID     string
	CompareSeparator string
	MergeBase        string
	Commits          []*git.Commit
	NumFiles         int
}

func (ci *CompareInfo) IsSameRepository() bool {
	return ci.BaseRepo.ID == ci.HeadRepo.ID
}

func (ci *CompareInfo) IsSameRef() bool {
	return ci.IsSameRepository() && ci.BaseRef == ci.HeadRef
}

func (ci *CompareInfo) DirectComparison() bool {
	// FIXME: the design of "DirectComparison" is wrong, it loses the information of `^`
	// To correctly handle the comparison, developers should use `ci.CompareSeparator` directly, all "DirectComparison" related code should be rewritten.
	return ci.CompareSeparator == ".."
}

// GetCompareInfo generates and returns compare information between base and head branches of repositories.
func GetCompareInfo(ctx context.Context, baseRepo, headRepo *repo_model.Repository, headGitRepo *git.Repository, baseRef, headRef git.RefName, directComparison, fileOnly bool) (_ *CompareInfo, err error) {
	compareInfo := &CompareInfo{
		BaseRepo:         baseRepo,
		BaseRef:          baseRef,
		HeadRepo:         headRepo,
		HeadGitRepo:      headGitRepo,
		HeadRef:          headRef,
		CompareSeparator: util.Iif(directComparison, "..", "..."),
	}

	compareInfo.BaseCommitID, err = gitrepo.GetFullCommitID(ctx, baseRepo, baseRef.String())
	if err != nil {
		return nil, err
	}
	compareInfo.HeadCommitID, err = gitrepo.GetFullCommitID(ctx, headRepo, headRef.String())
	if err != nil {
		return nil, err
	}

	// if they are not the same repository, then we need to fetch the base commit into the head repository
	// because we will use headGitRepo in the following code
	if baseRepo.ID != headRepo.ID {
		exist := headGitRepo.IsReferenceExist(compareInfo.BaseCommitID)
		if !exist {
			if err := gitrepo.FetchRemoteCommit(ctx, headRepo, baseRepo, compareInfo.BaseCommitID); err != nil {
				return nil, fmt.Errorf("FetchRemoteCommit: %w", err)
			}
		}
	}

	if !directComparison {
		compareInfo.MergeBase, err = gitrepo.MergeBase(ctx, headRepo, compareInfo.BaseCommitID, compareInfo.HeadCommitID)
		if err != nil {
			return nil, fmt.Errorf("MergeBase: %w", err)
		}
	} else {
		compareInfo.MergeBase = compareInfo.BaseCommitID
	}

	// We have a common base - therefore we know that ... should work
	if !fileOnly {
		// In git log/rev-list, the "..." syntax represents the symmetric difference between two references,
		// which is different from the meaning of "..." in git diff (where it implies diffing from the merge base).
		// For listing PR commits, we must use merge-base..head to include only the commits introduced by the head branch.
		// Otherwise, commits newly pushed to the base branch would also be included, which is incorrect.
		compareInfo.Commits, err = headGitRepo.ShowPrettyFormatLogToList(ctx, compareInfo.MergeBase+".."+compareInfo.HeadCommitID)
		if err != nil {
			return nil, fmt.Errorf("ShowPrettyFormatLogToList: %w", err)
		}
	} else {
		compareInfo.Commits = []*git.Commit{}
	}

	// Count number of changed files.
	// This probably should be removed as we need to use shortstat elsewhere
	// Now there is git diff --shortstat but this appears to be slower than simply iterating with --nameonly
	compareInfo.NumFiles, err = headGitRepo.GetDiffNumChangedFiles(compareInfo.BaseCommitID, compareInfo.HeadCommitID, directComparison)
	if err != nil {
		return nil, err
	}
	return compareInfo, nil
}
