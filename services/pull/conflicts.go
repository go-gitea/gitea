// Copyright 2025 The Gitea Authors. All rights reserved.
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

// checkPullRequestMergeableAndUpdateStatus checks whether a pull request is mergeable and updates its status accordingly.
// It uses 'git merge-tree' if supported by the Git version, otherwise it falls back to using a temporary repository.
// This function updates the pr.Status, pr.MergeBase and pr.ConflictedFiles fields as necessary.
func checkPullRequestMergeableAndUpdateStatus(ctx context.Context, pr *issues_model.PullRequest) error {
	if git.DefaultFeatures().SupportGitMergeTree {
		return checkPullRequestMergeableAndUpdateStatusMergeTree(ctx, pr)
	}

	return checkPullRequestMergeableAndUpdateStatusTmpRepo(ctx, pr)
}

// checkConflictsMergeTree uses git merge-tree to check for conflicts and if none are found checks if the patch is empty
// return true if there is conflicts otherwise return false
// pr.Status and pr.ConflictedFiles will be updated as necessary
func checkConflictsMergeTree(ctx context.Context, repoPath string, pr *issues_model.PullRequest, baseCommitID string) (bool, error) {
	treeHash, conflict, conflictFiles, err := gitrepo.MergeTree(ctx, pr.BaseRepo, pr.MergeBase, baseCommitID, pr.HeadCommitID)
	if err != nil {
		return false, fmt.Errorf("MergeTree: %w", err)
	}
	if conflict {
		pr.Status = issues_model.PullRequestStatusConflict
		pr.ConflictedFiles = conflictFiles

		log.Trace("Found %d files conflicted: %v", len(pr.ConflictedFiles), pr.ConflictedFiles)
		return true, nil
	}

	// No conflicts were detected, now check if the pull request actually
	// contains anything useful via a diff. git-diff-tree(1) with --quiet
	// will return exit code 0 if there's no diff and exit code 1 if there's
	// a diff.
	isEmpty := true
	if err = gitcmd.NewCommand("diff-tree", "--quiet").AddDynamicArguments(treeHash, pr.MergeBase).
		Run(ctx, &gitcmd.RunOpts{
			Dir: repoPath,
		}); err != nil {
		if !gitcmd.IsErrorExitCode(err, 1) {
			return false, fmt.Errorf("DiffTree: %w", err)
		}
		isEmpty = false
	}

	if isEmpty {
		log.Debug("PullRequest[%d]: Patch is empty - ignoring", pr.ID)
		pr.Status = issues_model.PullRequestStatusEmpty
	}
	return false, nil
}

func checkPullRequestMergeableAndUpdateStatusMergeTree(ctx context.Context, pr *issues_model.PullRequest) error {
	// 1. Get head commit
	if err := pr.LoadHeadRepo(ctx); err != nil {
		return err
	}
	headGitRepo, err := gitrepo.OpenRepository(ctx, pr.HeadRepo)
	if err != nil {
		return fmt.Errorf("OpenRepository: %w", err)
	}
	defer headGitRepo.Close()

	if pr.Flow == issues_model.PullRequestFlowGithub {
		pr.HeadCommitID, err = headGitRepo.GetRefCommitID(git.BranchPrefix + pr.HeadBranch)
		if err != nil {
			return fmt.Errorf("GetBranchCommitID: can't find commit ID for head: %w", err)
		}
	} else if pr.HeadCommitID == "" {
		return errors.New("head commit ID is empty for pull request Agit flow")
	}

	// 2. Get base commit id
	var baseGitRepo *git.Repository
	if pr.IsSameRepo() {
		baseGitRepo = headGitRepo
	} else {
		baseGitRepo, err = gitrepo.OpenRepository(ctx, pr.BaseRepo)
		if err != nil {
			return fmt.Errorf("OpenRepository: %w", err)
		}
		defer baseGitRepo.Close()
		// 2.1. fetch head commit id into the current repository
		// it will be checked in 2 weeks by default from git if the pull request created failure.
		if err := gitrepo.FetchRemoteCommit(ctx, pr.BaseRepo, pr.HeadRepo, pr.HeadCommitID); err != nil {
			return fmt.Errorf("FetchRemoteCommit: %w", err)
		}
	}
	baseCommitID, err := baseGitRepo.GetRefCommitID(git.BranchPrefix + pr.BaseBranch)
	if err != nil {
		return fmt.Errorf("GetBranchCommitID: can't find commit ID for base: %w", err)
	}

	// 3. update merge base
	pr.MergeBase, err = gitrepo.MergeBase(ctx, pr.BaseRepo, baseCommitID, pr.HeadCommitID)
	if err != nil {
		log.Error("GetMergeBase: %v and can't find commit ID for base: %v", err, baseCommitID)
		pr.Status = issues_model.PullRequestStatusEmpty // if there is no merge base, then it's empty but we still need to allow the pull request created
		return nil
	}

	// 4. if base == head, then it's an ancestor
	if pr.HeadCommitID == pr.MergeBase {
		pr.Status = issues_model.PullRequestStatusAncestor
		return nil
	}

	// 5. Check for conflicts
	conflicted, err := checkConflictsMergeTree(ctx, pr.BaseRepo.RepoPath(), pr, baseCommitID)
	if err != nil {
		log.Error("checkConflictsMergeTree: %v", err)
		pr.Status = issues_model.PullRequestStatusEmpty // if there is no merge base, then it's empty but we still need to allow the pull request created
	}
	if conflicted || pr.Status == issues_model.PullRequestStatusEmpty {
		return nil
	}

	// 6. Check for protected files changes
	if err = checkPullFilesProtection(ctx, pr, pr.BaseRepo.RepoPath()); err != nil {
		return fmt.Errorf("pr.CheckPullFilesProtection(): %v", err)
	}
	if len(pr.ChangedProtectedFiles) > 0 {
		log.Trace("Found %d protected files changed", len(pr.ChangedProtectedFiles))
	}

	pr.Status = issues_model.PullRequestStatusMergeable
	return nil
}
