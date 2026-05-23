// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"context"
	"errors"
	"fmt"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/log"
)

// checkConflictsMergeTree uses git merge-tree to check for conflicts and if none are found checks if the patch is empty
// return true if there are conflicts otherwise return false
// pr.Status and pr.ConflictedFiles will be updated as necessary
func checkConflictsMergeTree(ctx context.Context, pr *issues_model.PullRequest, baseCommitID string) (bool, error) {
	treeHash, conflict, conflictFiles, err := gitrepo.MergeTree(ctx, pr.BaseRepo, baseCommitID, pr.HeadCommitID, pr.MergeBase)
	if err != nil {
		return false, fmt.Errorf("MergeTree: %w", err)
	}
	if conflict {
		pr.Status = issues_model.PullRequestStatusConflict
		// sometimes git merge-tree will detect conflicts but not list any conflicted files
		// so that pr.ConflictedFiles will be empty
		pr.ConflictedFiles = conflictFiles

		log.Trace("Found %d files conflicted: %v", len(pr.ConflictedFiles), pr.ConflictedFiles)
		return true, nil
	}

	// Detect whether the pull request introduces changes by comparing the merged tree (treeHash)
	// against the current base commit (baseCommitID) using `git diff-tree`. The command returns exit code 0
	// if there is no diff between these trees (empty patch) and exit code 1 if there is a diff.
	gitErr := gitrepo.RunCmd(ctx, pr.BaseRepo, gitcmd.NewCommand("diff-tree", "-r", "--quiet").
		AddDynamicArguments(treeHash, baseCommitID))
	switch {
	case gitErr == nil:
		log.Debug("PullRequest[%d]: Patch is empty - ignoring", pr.ID)
		pr.Status = issues_model.PullRequestStatusEmpty
	case gitcmd.IsErrorExitCode(gitErr, 1):
		pr.Status = issues_model.PullRequestStatusMergeable
	default:
		return false, fmt.Errorf("run diff-tree exit abnormally: %w", gitErr)
	}
	return false, nil
}

func checkPullRequestMergeableByMergeTree(ctx context.Context, pr *issues_model.PullRequest) error {
	// 1. Get head commit
	if err := pr.LoadHeadRepo(ctx); err != nil {
		return err
	}
	headGitRepo, err := gitrepo.OpenRepository(ctx, pr.HeadRepo)
	if err != nil {
		return fmt.Errorf("OpenRepository: %w", err)
	}
	defer headGitRepo.Close()

	// 2. Get/open base repository
	var baseGitRepo *git.Repository
	if pr.IsSameRepo() {
		baseGitRepo = headGitRepo
	} else {
		baseGitRepo, err = gitrepo.OpenRepository(ctx, pr.BaseRepo)
		if err != nil {
			return fmt.Errorf("OpenRepository: %w", err)
		}
		defer baseGitRepo.Close()
	}

	// 3. Get head commit id
	if pr.Flow == issues_model.PullRequestFlowGithub {
		pr.HeadCommitID, err = headGitRepo.GetRefCommitID(git.BranchPrefix + pr.HeadBranch)
		if err != nil {
			return fmt.Errorf("GetBranchCommitID: can't find commit ID for head: %w", err)
		}
	} else {
		if pr.ID > 0 {
			pr.HeadCommitID, err = baseGitRepo.GetRefCommitID(pr.GetGitHeadRefName())
			if err != nil {
				return fmt.Errorf("GetRefCommitID: can't find commit ID for head: %w", err)
			}
		} else if pr.HeadCommitID == "" { // for new pull request with agit, the head commit id must be provided
			return errors.New("head commit ID is empty for pull request Agit flow")
		}
	}

	// 4. fetch head commit id into the current repository
	// it will be checked in 2 weeks by default from git if the pull request created failure.
	if !pr.IsSameRepo() {
		if !baseGitRepo.IsReferenceExist(pr.HeadCommitID) {
			if err := gitrepo.FetchRemoteCommit(ctx, pr.BaseRepo, pr.HeadRepo, pr.HeadCommitID); err != nil {
				return fmt.Errorf("FetchRemoteCommit: %w", err)
			}
		}
	}

	// 5. update merge base
	baseCommitID, err := baseGitRepo.GetRefCommitID(git.BranchPrefix + pr.BaseBranch)
	if err != nil {
		return fmt.Errorf("GetBranchCommitID: can't find commit ID for base: %w", err)
	}

	pr.MergeBase, err = gitrepo.MergeBase(ctx, pr.BaseRepo, baseCommitID, pr.HeadCommitID)
	if err != nil {
		// if there is no merge base, then it's empty, still need to allow the pull request to be created
		// not quite right (e.g.: why not reset the fields like below), but no interest to do more investigation at the moment
		log.Error("MergeBase: unable to find merge base between %s and %s: %v", baseCommitID, pr.HeadCommitID, err)
		pr.Status = issues_model.PullRequestStatusEmpty
		return nil
	}

	// reset conflicted files and changed protected files
	pr.ConflictedFiles = nil
	pr.ChangedProtectedFiles = nil

	// 6. if base == head, then it's an ancestor
	if pr.HeadCommitID == pr.MergeBase {
		pr.Status = issues_model.PullRequestStatusAncestor
		return nil
	}

	// 7. Check for conflicts
	conflicted, err := checkConflictsMergeTree(ctx, pr, baseCommitID)
	if err != nil {
		log.Error("checkConflictsMergeTree: %v", err)
		pr.Status = issues_model.PullRequestStatusError
		return fmt.Errorf("checkConflictsMergeTree: %w", err)
	}
	if conflicted || pr.Status == issues_model.PullRequestStatusEmpty {
		return nil
	}

	// 8. Check for protected files changes
	if err = checkPullFilesProtection(ctx, pr, baseGitRepo, pr.HeadCommitID); err != nil {
		return fmt.Errorf("checkPullFilesProtection: %w", err)
	}
	return nil
}
