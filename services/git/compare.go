// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"
	"strconv"
	"time"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/graceful"
	logger "code.gitea.io/gitea/modules/log"
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

func (ci *CompareInfo) Separator() string {
	if ci.DirectComparison {
		return ".."
	}
	return "..."
}

func (ci *CompareInfo) OtherSeparator() string {
	if ci.DirectComparison {
		return "..."
	}
	return ".."
}

// GetCompareInfo generates and returns compare information between base and head branches of repositories.
func GetCompareInfo(ctx context.Context, baseRepo, headRepo *repo_model.Repository, headGitRepo *git.Repository, baseRef, headRef git.RefName, directComparison, fileOnly bool) (_ *CompareInfo, err error) {
	var tmpRemote string

	// We don't need a temporary remote for same repository.
	if baseRepo.ID != headRepo.ID {
		// Add a temporary remote
		tmpRemote = strconv.FormatInt(time.Now().UnixNano(), 10)
		if err = gitrepo.GitRemoteAdd(ctx, headRepo, tmpRemote, baseRepo.RepoPath()); err != nil {
			return nil, fmt.Errorf("GitRemoteAdd: %w", err)
		}
		defer func() {
			if err := gitrepo.GitRemoteRemove(graceful.GetManager().ShutdownContext(), headRepo, tmpRemote); err != nil {
				logger.Error("GetPullRequestInfo: GitRemoteRemove: %v", err)
			}
		}()
	}

	compareInfo := &CompareInfo{
		BaseRepo:         baseRepo,
		BaseRef:          baseRef,
		HeadRepo:         headRepo,
		HeadGitRepo:      headGitRepo,
		HeadRef:          headRef,
		DirectComparison: directComparison,
	}

	compareInfo.HeadCommitID, err = gitrepo.GetFullCommitID(ctx, headRepo, headRef.String())
	if err != nil {
		compareInfo.HeadCommitID = headRef.String()
	}

	compareInfo.BaseCommitID, err = gitrepo.GetFullCommitID(ctx, baseRepo, baseRef.String())
	if err != nil {
		compareInfo.BaseCommitID = baseRef.String()
	}

	compareInfo.MergeBase, _, err = headGitRepo.GetMergeBase(tmpRemote, compareInfo.BaseCommitID, compareInfo.HeadCommitID)
	if err != nil {
		return nil, fmt.Errorf("GetMergeBase: %w", err)
	}

	// We have a common base - therefore we know that ... should work
	if !fileOnly {
		compareInfo.Commits, err = headGitRepo.ShowPrettyFormatLogToList(ctx, compareInfo.BaseCommitID+compareInfo.Separator()+compareInfo.HeadCommitID)
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
