// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
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
	DirectComparison bool
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

// GetCompareInfo generates and returns compare information between base and head branches of repositories.
func GetCompareInfo(ctx context.Context, baseRepo, headRepo *repo_model.Repository, headGitRepo *git.Repository, baseRef, headRef git.RefName, directComparison, fileOnly bool) (_ *CompareInfo, err error) {
	compareInfo := new(CompareInfo)

	compareInfo.HeadCommitID, err = gitrepo.GetFullCommitID(ctx, headRepo, headRef.String())
	if err != nil {
		compareInfo.HeadCommitID = headRef.String()
	}
	compareInfo.BaseCommitID, err = gitrepo.GetFullCommitID(ctx, baseRepo, baseRef.String())
	if err != nil {
		compareInfo.BaseCommitID = baseRef.String()
	}

	compareInfo.MergeBase, err = gitrepo.MergeBaseFromRemote(ctx, baseRepo, headRepo, compareInfo.BaseCommitID, compareInfo.HeadCommitID)
	if err == nil {
		separator := "..."
		startCommitID := compareInfo.MergeBase
		if directComparison {
			separator = ".."
			startCommitID = compareInfo.BaseCommitID
		}

		// We have a common base - therefore we know that ... should work
		if !fileOnly {
			compareInfo.Commits, err = headGitRepo.ShowPrettyFormatLogToList(ctx, startCommitID+separator+headRef.String())
			if err != nil {
				return nil, fmt.Errorf("ShowPrettyFormatLogToList: %w", err)
			}
		} else {
			compareInfo.Commits = []*git.Commit{}
		}
	} else {
		compareInfo.Commits = []*git.Commit{}
		compareInfo.MergeBase = compareInfo.BaseCommitID
	}

	if baseRepo.ID != headRepo.ID {
		if err := gitrepo.FetchRemoteCommit(ctx, headRepo, baseRepo, compareInfo.BaseCommitID); err != nil {
			return nil, fmt.Errorf("FetchRemoteCommit: %w", err)
		}
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
