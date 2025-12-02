// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

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
	MergeBase    string
	BaseCommitID string
	HeadCommitID string
	Commits      []*git.Commit
	NumFiles     int
}

// GetCompareInfo generates and returns compare information between base and head branches of repositories.
func GetCompareInfo(ctx context.Context, baseRepo, headRepo *repo_model.Repository, headGitRepo *git.Repository, baseBranch, headBranch string, directComparison, fileOnly bool) (_ *CompareInfo, err error) {
	var (
		remoteBranch string
		tmpRemote    string
	)

	// We don't need a temporary remote for same repository.
	if headGitRepo.Path != baseRepo.RepoPath() {
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

	compareInfo := new(CompareInfo)

	compareInfo.HeadCommitID, err = git.GetFullCommitID(ctx, headGitRepo.Path, headBranch)
	if err != nil {
		compareInfo.HeadCommitID = headBranch
	}

	compareInfo.MergeBase, remoteBranch, err = headGitRepo.GetMergeBase(tmpRemote, baseBranch, headBranch)
	if err == nil {
		compareInfo.BaseCommitID, err = git.GetFullCommitID(ctx, headGitRepo.Path, remoteBranch)
		if err != nil {
			compareInfo.BaseCommitID = remoteBranch
		}
		separator := "..."
		baseCommitID := compareInfo.MergeBase
		if directComparison {
			separator = ".."
			baseCommitID = compareInfo.BaseCommitID
		}

		// We have a common base - therefore we know that ... should work
		if !fileOnly {
			compareInfo.Commits, err = headGitRepo.ShowPrettyFormatLogToList(ctx, baseCommitID+separator+headBranch)
			if err != nil {
				return nil, fmt.Errorf("ShowPrettyFormatLogToList: %w", err)
			}
		} else {
			compareInfo.Commits = []*git.Commit{}
		}
	} else {
		compareInfo.Commits = []*git.Commit{}
		compareInfo.MergeBase, err = git.GetFullCommitID(ctx, headGitRepo.Path, remoteBranch)
		if err != nil {
			compareInfo.MergeBase = remoteBranch
		}
		compareInfo.BaseCommitID = compareInfo.MergeBase
	}

	// Count number of changed files.
	// This probably should be removed as we need to use shortstat elsewhere
	// Now there is git diff --shortstat but this appears to be slower than simply iterating with --nameonly
	compareInfo.NumFiles, err = headGitRepo.GetDiffNumChangedFiles(remoteBranch, headBranch, directComparison)
	if err != nil {
		return nil, err
	}
	return compareInfo, nil
}
